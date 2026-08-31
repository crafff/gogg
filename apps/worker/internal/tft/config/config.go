package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

var SupportedPlatforms = []string{
	"NA1", "BR1", "LA1", "LA2",
	"KR", "JP1",
	"EUN1", "EUW1", "TR1", "ME1", "RU",
	"OC1", "SG2", "TW2", "VN2",
}

type Config struct {
	Temporal TemporalConfig `mapstructure:"temporal"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Riot     RiotConfig     `mapstructure:"riot"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Raw      RawConfig      `mapstructure:"raw_archive"`
	TFT      TFTConfig      `mapstructure:"tft"`
}

type TemporalConfig struct {
	HostPort  string `mapstructure:"host_port"`
	Namespace string `mapstructure:"namespace"`
}

type DatabaseConfig struct {
	DSN             string `mapstructure:"dsn"`
	MaxOpenConns    int32  `mapstructure:"max_open_conns"`
	MaxIdleConns    int32  `mapstructure:"max_idle_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime_seconds"`
}

type RedisConfig struct {
	URL string `mapstructure:"url"`
}
type RiotConfig struct {
	APIKey string `mapstructure:"api_key"`
}
type LoggingConfig struct{ Level, Format string }

type RawConfig struct {
	Root             string `mapstructure:"root"`
	CompressionLevel int    `mapstructure:"compression_level"`
}

type TFTConfig struct {
	Platforms               []string      `mapstructure:"platforms"`
	CrawlCron               string        `mapstructure:"crawl_cron"`
	StaticCron              string        `mapstructure:"static_cron"`
	ProfileName             string        `mapstructure:"profile_name"`
	MatchCountPerSeed       int           `mapstructure:"match_count_per_seed"`
	MatchTargetPerRegion    int           `mapstructure:"match_target_per_region"`
	MatchSelectionRevision  string        `mapstructure:"match_selection_revision"`
	MasterLimit             int           `mapstructure:"master_limit"`
	DiamondPerDivision      int           `mapstructure:"diamond_per_division"`
	ScaleMasterLimit        int           `mapstructure:"scale_master_limit"`
	ScaleDiamondPerDivision int           `mapstructure:"scale_diamond_per_division"`
	ScaleAfter              time.Duration `mapstructure:"scale_after"`
	ScaleBelowObservations  int           `mapstructure:"scale_below_observations"`
	Window                  time.Duration `mapstructure:"window"`
	WindowLag               time.Duration `mapstructure:"window_lag"`
	Overlap                 time.Duration `mapstructure:"overlap"`
	StaticRoot              string        `mapstructure:"static_root"`
	StaticLocales           []string      `mapstructure:"static_locales"`
	StaticDownloadBatch     int           `mapstructure:"static_download_batch"`
	StaticDownloadWorkers   int           `mapstructure:"static_download_workers"`
}

func Default() Config {
	return Config{
		Temporal: TemporalConfig{HostPort: "localhost:7233", Namespace: "default"},
		Database: DatabaseConfig{DSN: "postgres://gogg:goggpass@localhost:55433/gogg?sslmode=disable", MaxOpenConns: 10, MaxIdleConns: 2, ConnMaxLifetime: 300},
		Redis:    RedisConfig{URL: "redis://localhost:6379/0"},
		Logging:  LoggingConfig{Level: "info", Format: "json"},
		Raw:      RawConfig{Root: "data/riot-raw", CompressionLevel: 6},
		TFT: TFTConfig{
			Platforms: append([]string(nil), SupportedPlatforms...), CrawlCron: "17 */3 * * *", StaticCron: "7 */6 * * *",
			ProfileName: "global_high_tier", MatchCountPerSeed: 20, MatchTargetPerRegion: 1000,
			MatchSelectionRevision: "route-balance-v1", MasterLimit: 500, DiamondPerDivision: 50,
			ScaleMasterLimit: 1000, ScaleDiamondPerDivision: 100, ScaleAfter: 48 * time.Hour,
			ScaleBelowObservations: 10000, Window: 7 * 24 * time.Hour, WindowLag: 30 * time.Minute,
			Overlap: 6 * time.Hour, StaticRoot: "data/game-assets", StaticLocales: []string{"en_us", "zh_cn"},
			StaticDownloadBatch: 128, StaticDownloadWorkers: 16,
		},
	}
}

func Load() (Config, error) {
	cfg := Default()
	v := viper.New()
	bindDefaults(v, cfg)
	path := os.Getenv("APP_CONFIG_PATH")
	required := path != ""
	if path == "" {
		path = "config/dev.yaml"
	}
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if required || (!errors.As(err, &notFound) && !os.IsNotExist(err)) {
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
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	var errs []error
	if strings.TrimSpace(c.Riot.APIKey) == "" || c.Riot.APIKey == "YOUR_API_KEY" {
		errs = append(errs, fmt.Errorf("riot.api_key is required"))
	}
	if strings.TrimSpace(c.Database.DSN) == "" {
		errs = append(errs, fmt.Errorf("database.dsn is required"))
	}
	if strings.TrimSpace(c.Redis.URL) == "" {
		errs = append(errs, fmt.Errorf("redis.url is required"))
	}
	if strings.TrimSpace(c.Raw.Root) == "" {
		errs = append(errs, fmt.Errorf("raw_archive.root is required"))
	}
	if c.Raw.CompressionLevel < 0 || c.Raw.CompressionLevel > 9 {
		errs = append(errs, fmt.Errorf("raw_archive.compression_level must be 0..9"))
	}
	if len(c.TFT.Platforms) == 0 {
		errs = append(errs, fmt.Errorf("tft.platforms must not be empty"))
	}
	supported := make(map[string]bool, len(SupportedPlatforms))
	for _, p := range SupportedPlatforms {
		supported[p] = true
	}
	for _, p := range c.TFT.Platforms {
		if !supported[strings.ToUpper(p)] {
			errs = append(errs, fmt.Errorf("unsupported TFT platform %q", p))
		}
	}
	hasEnglishStatic := false
	for _, locale := range c.TFT.StaticLocales {
		if strings.EqualFold(locale, "en_us") {
			hasEnglishStatic = true
			break
		}
	}
	if !hasEnglishStatic {
		errs = append(errs, fmt.Errorf("tft.static_locales must include en_us"))
	}
	if c.TFT.StaticDownloadBatch < 1 || c.TFT.StaticDownloadBatch > 1000 {
		errs = append(errs, fmt.Errorf("tft.static_download_batch must be 1..1000"))
	}
	if c.TFT.StaticDownloadWorkers < 1 || c.TFT.StaticDownloadWorkers > 64 {
		errs = append(errs, fmt.Errorf("tft.static_download_workers must be 1..64"))
	} else if c.TFT.StaticDownloadBatch > c.TFT.StaticDownloadWorkers*8 {
		errs = append(errs, fmt.Errorf("tft.static_download_batch must not exceed 8 times static_download_workers"))
	}
	if c.TFT.MatchCountPerSeed < 1 || c.TFT.MatchCountPerSeed > 100 {
		errs = append(errs, fmt.Errorf("tft.match_count_per_seed must be 1..100"))
	}
	if c.TFT.MatchTargetPerRegion < 1 || c.TFT.MatchTargetPerRegion > 100000 {
		errs = append(errs, fmt.Errorf("tft.match_target_per_region must be 1..100000"))
	}
	if strings.TrimSpace(c.TFT.MatchSelectionRevision) == "" {
		errs = append(errs, fmt.Errorf("tft.match_selection_revision must not be empty"))
	}
	if c.TFT.Window <= 0 || c.TFT.WindowLag < 0 || c.TFT.Overlap < 0 {
		errs = append(errs, fmt.Errorf("invalid TFT collection window"))
	}
	return errors.Join(errs...)
}

func bindDefaults(v *viper.Viper, d Config) {
	v.SetDefault("temporal.host_port", d.Temporal.HostPort)
	v.SetDefault("temporal.namespace", d.Temporal.Namespace)
	v.SetDefault("database.dsn", d.Database.DSN)
	v.SetDefault("database.max_open_conns", d.Database.MaxOpenConns)
	v.SetDefault("database.max_idle_conns", d.Database.MaxIdleConns)
	v.SetDefault("database.conn_max_lifetime_seconds", d.Database.ConnMaxLifetime)
	v.SetDefault("redis.url", d.Redis.URL)
	v.SetDefault("riot.api_key", d.Riot.APIKey)
	v.SetDefault("raw_archive.root", d.Raw.Root)
	v.SetDefault("raw_archive.compression_level", d.Raw.CompressionLevel)
	v.SetDefault("logging.level", d.Logging.Level)
	v.SetDefault("logging.format", d.Logging.Format)
	v.SetDefault("tft.platforms", d.TFT.Platforms)
	v.SetDefault("tft.crawl_cron", d.TFT.CrawlCron)
	v.SetDefault("tft.static_cron", d.TFT.StaticCron)
	v.SetDefault("tft.profile_name", d.TFT.ProfileName)
	v.SetDefault("tft.match_count_per_seed", d.TFT.MatchCountPerSeed)
	v.SetDefault("tft.match_target_per_region", d.TFT.MatchTargetPerRegion)
	v.SetDefault("tft.match_selection_revision", d.TFT.MatchSelectionRevision)
	v.SetDefault("tft.master_limit", d.TFT.MasterLimit)
	v.SetDefault("tft.diamond_per_division", d.TFT.DiamondPerDivision)
	v.SetDefault("tft.scale_master_limit", d.TFT.ScaleMasterLimit)
	v.SetDefault("tft.scale_diamond_per_division", d.TFT.ScaleDiamondPerDivision)
	v.SetDefault("tft.scale_after", d.TFT.ScaleAfter)
	v.SetDefault("tft.scale_below_observations", d.TFT.ScaleBelowObservations)
	v.SetDefault("tft.window", d.TFT.Window)
	v.SetDefault("tft.window_lag", d.TFT.WindowLag)
	v.SetDefault("tft.overlap", d.TFT.Overlap)
	v.SetDefault("tft.static_root", d.TFT.StaticRoot)
	v.SetDefault("tft.static_locales", d.TFT.StaticLocales)
	v.SetDefault("tft.static_download_batch", d.TFT.StaticDownloadBatch)
	v.SetDefault("tft.static_download_workers", d.TFT.StaticDownloadWorkers)
}

func PlatformURL(platform string) string {
	return "https://" + strings.ToLower(platform) + ".api.riotgames.com"
}

func RoutingRegion(platform string) string {
	switch strings.ToUpper(platform) {
	case "NA1", "BR1", "LA1", "LA2":
		return "AMERICAS"
	case "KR", "JP1":
		return "ASIA"
	case "EUN1", "EUW1", "TR1", "ME1", "RU":
		return "EUROPE"
	case "OC1", "SG2", "TW2", "VN2":
		return "SEA"
	default:
		return ""
	}
}

func RegionalURL(platform string) string {
	return "https://" + strings.ToLower(RoutingRegion(platform)) + ".api.riotgames.com"
}
