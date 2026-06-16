// Package config loads and validates the gogg-worker configuration.
//
// The worker hosts Temporal workflows + activities. Phase C ships
// chunk-by-chunk: chunk 1 only needs Temporal + logging — chunks 2+
// will extend this struct with Database + RiotAPI as those Activities
// land. Keeping the surface small at chunk 1 makes the smoke test
// boundary obvious.
//
// Layering, highest precedence last:
//
//  1. Defaults baked into Default().
//  2. YAML file pointed at by APP_CONFIG_PATH (default: ./config.yaml).
//  3. Environment variables prefixed with GOGG_ (e.g.
//     GOGG_TEMPORAL_HOST_PORT, GOGG_TEMPORAL_NAMESPACE).
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config is the full runtime configuration for gogg-worker.
type Config struct {
	Temporal TemporalConfig `mapstructure:"temporal"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Crawler  CrawlerConfig  `mapstructure:"crawler"`
}

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
		Crawler: CrawlerConfig{
			ConfigPath: "config.yaml",
		},
	}
}

// CrawlerConfig points at the legacy YAML (internal/config schema) that
// holds riot.api_key, regions[], database.dsn, run_profiles. Worker
// loads it via internal/config.Load at startup. Keeping the legacy
// loader avoids re-deriving the per-region routing URL + the profile
// validation already proven in production.
type CrawlerConfig struct {
	ConfigPath string `mapstructure:"config_path"`
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
		path = "config.yaml"
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
	if strings.TrimSpace(c.Crawler.ConfigPath) == "" {
		errs = append(errs, fmt.Errorf("crawler.config_path is required"))
	}
	return errors.Join(errs...)
}

func bindDefaults(v *viper.Viper, def Config) error {
	v.SetDefault("temporal.host_port", def.Temporal.HostPort)
	v.SetDefault("temporal.namespace", def.Temporal.Namespace)
	v.SetDefault("temporal.task_queues", def.Temporal.TaskQueues)
	v.SetDefault("logging.level", def.Logging.Level)
	v.SetDefault("logging.format", def.Logging.Format)
	v.SetDefault("crawler.config_path", def.Crawler.ConfigPath)
	return nil
}
