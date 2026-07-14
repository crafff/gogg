package config

import "time"

// RiotConfig holds a global API key and a default base URL used when
// no per-region config is defined.
type RiotConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// RegionConfig describes one Riot API platform region.
type RegionConfig struct {
	Name    string `mapstructure:"name"`    // e.g. "KR", "NA1", "EUW1"
	APIKey  string `mapstructure:"api_key"` // overrides riot.api_key
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

type CrawlerLiteConfig struct {
	OutageInitialInterval time.Duration `mapstructure:"outage_initial_interval"`
	OutageMaxInterval     time.Duration `mapstructure:"outage_max_interval"`
	OutageJitter          float64       `mapstructure:"outage_jitter"`
}

type ScheduleEntry struct {
	Cron    string `mapstructure:"cron"`
	Profile string `mapstructure:"profile"`
}
