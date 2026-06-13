package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Riot     RiotConfig              `mapstructure:"riot"`
	Regions  []RegionConfig          `mapstructure:"regions"`
	Database DatabaseConfig          `mapstructure:"database"`
	Crawler  CrawlerConfig           `mapstructure:"crawler"`
	Schedule []ScheduleEntry         `mapstructure:"schedule"`
	Profiles map[string]RunProfile   `mapstructure:"run_profiles"`
}

// RiotConfig holds a global API key and a default base URL (used when no
// per-region config is defined — backward-compatible single-region setup).
type RiotConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// RegionConfig describes one Riot API region.
type RegionConfig struct {
	Name    string `mapstructure:"name"`    // e.g. "KR", "NA1", "EUW1"
	APIKey  string `mapstructure:"api_key"` // overrides riot.api_key for this region
	BaseURL string `mapstructure:"base_url"`
}

type DatabaseConfig struct {
	DSN             string `mapstructure:"dsn"`
	MaxOpenConns    int32  `mapstructure:"max_open_conns"`
	MaxIdleConns    int32  `mapstructure:"max_idle_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime_seconds"`
}

type CrawlerConfig struct {
	Phase1PrefetchTiers []string `mapstructure:"phase1_prefetch_tiers"`
}

type ScheduleEntry struct {
	Cron    string `mapstructure:"cron"`
	Profile string `mapstructure:"profile"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	v.SetDefault("riot.base_url", "https://kr.api.riotgames.com")
	v.SetDefault("database.max_open_conns", 10)
	v.SetDefault("database.max_idle_conns", 2)
	v.SetDefault("database.conn_max_lifetime_seconds", 300)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	for _, r := range c.resolvedRegions() {
		if r.APIKey == "" || r.APIKey == "YOUR_API_KEY" {
			return fmt.Errorf("api_key for region %q is not configured", r.Name)
		}
		if r.BaseURL == "" {
			return fmt.Errorf("base_url for region %q is required", r.Name)
		}
	}
	if c.Database.DSN == "" {
		return fmt.Errorf("database.dsn is required")
	}
	return nil
}

// resolvedRegions returns the effective region list. If no explicit regions are
// configured, it synthesizes one from the legacy riot.base_url setting so that
// existing single-region configs continue to work without changes.
func (c *Config) resolvedRegions() []RegionConfig {
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

// RegionByName returns the RegionConfig for the given name (case-insensitive).
func (c *Config) RegionByName(name string) (RegionConfig, error) {
	for _, r := range c.resolvedRegions() {
		if strings.EqualFold(r.Name, name) {
			return r, nil
		}
	}
	return RegionConfig{}, fmt.Errorf("region %q not found in config", name)
}

// Profile returns the named profile, or an error if not found.
func (c *Config) Profile(name string) (*RunProfile, error) {
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