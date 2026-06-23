// Package config loads and validates the gogg-worker configuration.
//
// The worker hosts Temporal workflows + activities. A single config
// document owns both process settings (Temporal/logging) and crawler
// settings (Riot regions, database, schedules, run profiles).
//
// Layering, highest precedence last:
//
//  1. Defaults baked into Default().
//  2. YAML file pointed at by APP_CONFIG_PATH (default: ./config/dev.yaml).
//  3. Environment variables prefixed with GOGG_ (e.g.
//     GOGG_TEMPORAL_HOST_PORT, GOGG_TEMPORAL_NAMESPACE).
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	crawlercfg "github.com/crafff/gogg/apps/worker/internal/crawlerconfig"
	"github.com/spf13/viper"
)

// Config is the full runtime configuration for gogg-worker.
type Config struct {
	Temporal TemporalConfig `mapstructure:"temporal"`
	Logging  LoggingConfig  `mapstructure:"logging"`

	Riot     RiotConfig            `mapstructure:"riot"`
	Regions  []RegionConfig        `mapstructure:"regions"`
	Database DatabaseConfig        `mapstructure:"database"`
	Crawler  CrawlerConfig         `mapstructure:"crawler"`
	Schedule []ScheduleEntry       `mapstructure:"schedule"`
	Profiles map[string]RunProfile `mapstructure:"run_profiles"`
}

type RiotConfig = crawlercfg.RiotConfig
type RegionConfig = crawlercfg.RegionConfig
type DatabaseConfig = crawlercfg.DatabaseConfig
type CrawlerConfig = crawlercfg.CrawlerConfig
type ScheduleEntry = crawlercfg.ScheduleEntry
type RunProfile = crawlercfg.RunProfile
type Mode = crawlercfg.Mode
type Execution = crawlercfg.Execution

const (
	ModeIncremental     = crawlercfg.ModeIncremental
	ModeHistorical      = crawlercfg.ModeHistorical
	ExecutionPipeline   = crawlercfg.ExecutionPipeline
	ExecutionSequential = crawlercfg.ExecutionSequential
)

// TemporalConfig wires the SDK client + the task queues this worker
// process subscribes to. Multiple queues let one process serve both
// crawl-kr and crawl-na1 in dev; production deploys typically run one
// worker per region so noisy-neighbor effects on the rate limiter stay
// scoped.
type TemporalConfig struct {
	HostPort   string   `mapstructure:"host_port"`
	Namespace  string   `mapstructure:"namespace"`
	TaskQueues []string `mapstructure:"task_queues"`
}

// LoggingConfig matches the api's LoggingConfig field-for-field so a
// shared package can absorb both later without churn. Kept duplicated
// for now to avoid a packages/logging detour during Phase C chunk 1.
type LoggingConfig struct {
	Level  string `mapstructure:"level"`  // debug | info | warn | error
	Format string `mapstructure:"format"` // json | text
}

// Default returns a Config populated for local dev against the compose
// stack (deploy/compose/docker-compose.dev.yml: temporal:7233).
// The default queue list pairs one queue per region so the same
// process can serve KR and NA1 in dev; production deploys typically
// run one worker per region.
func Default() Config {
	return Config{
		Temporal: TemporalConfig{
			HostPort:   "localhost:7233",
			Namespace:  "default",
			TaskQueues: []string{"crawl-kr", "crawl-na1"},
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}
}

// Load reads the config from disk + env and returns a validated Config.
func Load() (Config, error) {
	cfg := Default()
	v := viper.New()

	if err := bindDefaults(v, cfg); err != nil {
		return Config{}, fmt.Errorf("bind defaults: %w", err)
	}

	path := os.Getenv("APP_CONFIG_PATH")
	required := path != ""
	if path == "" {
		path = "config/dev.yaml"
	}
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		switch {
		case errors.As(err, &notFound), os.IsNotExist(err):
			if required {
				return Config{}, fmt.Errorf("config file %s: %w", path, err)
			}
		default:
			return Config{}, fmt.Errorf("read config %s: %w", path, err)
		}
	}

	v.SetEnvPrefix("GOGG")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate config: %w", err)
	}
	return cfg, nil
}

// Validate returns the first config error or nil.
func (c Config) Validate() error {
	var errs []error
	if strings.TrimSpace(c.Temporal.HostPort) == "" {
		errs = append(errs, fmt.Errorf("temporal.host_port is required"))
	}
	if strings.TrimSpace(c.Temporal.Namespace) == "" {
		errs = append(errs, fmt.Errorf("temporal.namespace is required"))
	}
	if len(c.Temporal.TaskQueues) == 0 {
		errs = append(errs, fmt.Errorf("temporal.task_queues must list at least one queue"))
	}
	for i, tq := range c.Temporal.TaskQueues {
		if strings.TrimSpace(tq) == "" {
			errs = append(errs, fmt.Errorf("temporal.task_queues[%d] is blank", i))
		}
	}
	switch c.Logging.Level {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, fmt.Errorf("logging.level %q: want debug|info|warn|error", c.Logging.Level))
	}
	switch c.Logging.Format {
	case "json", "text":
	default:
		errs = append(errs, fmt.Errorf("logging.format %q: want json|text", c.Logging.Format))
	}

	for _, r := range c.ResolvedRegions() {
		if r.APIKey == "" || r.APIKey == "YOUR_API_KEY" {
			errs = append(errs, fmt.Errorf("api_key for region %q is not configured", r.Name))
		}
		if strings.TrimSpace(r.BaseURL) == "" {
			errs = append(errs, fmt.Errorf("base_url for region %q is required", r.Name))
		}
	}
	if strings.TrimSpace(c.Database.DSN) == "" {
		errs = append(errs, fmt.Errorf("database.dsn is required"))
	}
	return errors.Join(errs...)
}

// ResolvedRegions returns the effective region list. Empty per-region
// API keys inherit the global Riot API key, and a single KR region is
// synthesized for older single-region config files.
func (c Config) ResolvedRegions() []RegionConfig {
	if len(c.Regions) > 0 {
		out := make([]RegionConfig, len(c.Regions))
		for i, r := range c.Regions {
			if r.APIKey == "" {
				r.APIKey = c.Riot.APIKey
			}
			out[i] = r
		}
		return out
	}
	return []RegionConfig{{
		Name:    "KR",
		APIKey:  c.Riot.APIKey,
		BaseURL: c.Riot.BaseURL,
	}}
}

// Profile returns the named crawler run profile.
func (c Config) Profile(name string) (*RunProfile, error) {
	p, ok := c.Profiles[name]
	if !ok {
		keys := make([]string, 0, len(c.Profiles))
		for k := range c.Profiles {
			keys = append(keys, k)
		}
		return nil, fmt.Errorf("profile %q not found (available: %s)", name, strings.Join(keys, ", "))
	}
	return &p, nil
}

func bindDefaults(v *viper.Viper, def Config) error {
	v.SetDefault("temporal.host_port", def.Temporal.HostPort)
	v.SetDefault("temporal.namespace", def.Temporal.Namespace)
	v.SetDefault("temporal.task_queues", def.Temporal.TaskQueues)
	v.SetDefault("logging.level", def.Logging.Level)
	v.SetDefault("logging.format", def.Logging.Format)
	v.SetDefault("riot.base_url", "https://kr.api.riotgames.com")
	v.SetDefault("database.max_open_conns", 10)
	v.SetDefault("database.max_idle_conns", 2)
	v.SetDefault("database.conn_max_lifetime_seconds", 300)
	return nil
}
