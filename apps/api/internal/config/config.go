// Package config loads and validates the gogg-api configuration.
//
// Layering, highest precedence last:
//
//  1. Defaults baked into Default().
//  2. YAML file pointed at by APP_CONFIG_PATH (default: ./config.yaml).
//  3. Environment variables prefixed with GOGG_ (e.g. GOGG_API_PORT,
//     GOGG_DATABASE_DSN).
//
// SOPS-encrypted secrets are expected to be decrypted into a plain
// YAML file before the binary starts (CI / deploy / `make run-api`
// handle this); the config loader itself never invokes sops.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config is the full runtime configuration for gogg-api.
type Config struct {
	API      APIConfig      `mapstructure:"api"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Logging  LoggingConfig  `mapstructure:"logging"`
}

// APIConfig controls the HTTP server.
type APIConfig struct {
	Port              int           `mapstructure:"port"`
	ReadTimeout       time.Duration `mapstructure:"read_timeout"`
	WriteTimeout      time.Duration `mapstructure:"write_timeout"`
	IdleTimeout       time.Duration `mapstructure:"idle_timeout"`
	ShutdownGrace     time.Duration `mapstructure:"shutdown_grace"`
	AllowedOrigins    []string      `mapstructure:"allowed_origins"`
	GraphQLPlayground bool          `mapstructure:"graphql_playground"`
}

// DatabaseConfig wires pgxpool.
type DatabaseConfig struct {
	DSN                    string        `mapstructure:"dsn"`
	MaxOpenConns           int           `mapstructure:"max_open_conns"`
	MinIdleConns           int           `mapstructure:"min_idle_conns"`
	ConnMaxLifetimeSeconds time.Duration `mapstructure:"conn_max_lifetime"`
}

// RedisConfig wires the cache client (used by service layer in later
// Phase B steps; included here so config-time validation surfaces
// missing URLs early).
type RedisConfig struct {
	URL string `mapstructure:"url"`
}

// LoggingConfig configures slog.
type LoggingConfig struct {
	Level  string `mapstructure:"level"`  // debug | info | warn | error
	Format string `mapstructure:"format"` // json | text
}

// Default returns a Config populated with safe defaults for local dev.
// Production values are expected to come from config.yaml + env vars.
func Default() Config {
	return Config{
		API: APIConfig{
			Port:              8080,
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      15 * time.Second,
			IdleTimeout:       60 * time.Second,
			ShutdownGrace:     15 * time.Second,
			AllowedOrigins:    []string{"http://localhost:5173", "http://localhost:3000"},
			GraphQLPlayground: true,
		},
		Database: DatabaseConfig{
			DSN:                    "postgres://gogg:goggpass@localhost:55433/gogg?sslmode=disable",
			MaxOpenConns:           10,
			MinIdleConns:           2,
			ConnMaxLifetimeSeconds: 5 * time.Minute,
		},
		Redis: RedisConfig{
			URL: "redis://localhost:6379/0",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}
}

// Load reads the config from disk + env and returns a validated Config.
// Returns an error wrapping all validation failures.
func Load() (Config, error) {
	cfg := Default()
	v := viper.New()

	// Defaults first so viper knows the shape.
	if err := bindDefaults(v, cfg); err != nil {
		return Config{}, fmt.Errorf("bind defaults: %w", err)
	}

	// YAML file is optional. If APP_CONFIG_PATH is set we require it
	// to exist; the default ./config.yaml is read on a best-effort basis.
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

	// Env overrides last. Format: GOGG_API_PORT, GOGG_DATABASE_DSN, etc.
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

// Validate returns the first config error or nil. We use errors.Join so
// startup logs surface every problem at once instead of trickling them
// out one boot at a time.
func (c Config) Validate() error {
	var errs []error
	if c.API.Port < 1 || c.API.Port > 65535 {
		errs = append(errs, fmt.Errorf("api.port %d out of range", c.API.Port))
	}
	if c.API.ReadTimeout <= 0 {
		errs = append(errs, fmt.Errorf("api.read_timeout must be > 0"))
	}
	if c.API.WriteTimeout <= 0 {
		errs = append(errs, fmt.Errorf("api.write_timeout must be > 0"))
	}
	if c.API.ShutdownGrace <= 0 {
		errs = append(errs, fmt.Errorf("api.shutdown_grace must be > 0"))
	}
	if strings.TrimSpace(c.Database.DSN) == "" {
		errs = append(errs, fmt.Errorf("database.dsn is required"))
	}
	if c.Database.MaxOpenConns <= 0 {
		errs = append(errs, fmt.Errorf("database.max_open_conns must be > 0"))
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
	return errors.Join(errs...)
}

// bindDefaults seeds viper with the Default() struct so YAML omission
// doesn't zero out fields the caller intended to keep.
func bindDefaults(v *viper.Viper, def Config) error {
	v.SetDefault("api.port", def.API.Port)
	v.SetDefault("api.read_timeout", def.API.ReadTimeout)
	v.SetDefault("api.write_timeout", def.API.WriteTimeout)
	v.SetDefault("api.idle_timeout", def.API.IdleTimeout)
	v.SetDefault("api.shutdown_grace", def.API.ShutdownGrace)
	v.SetDefault("api.allowed_origins", def.API.AllowedOrigins)
	v.SetDefault("api.graphql_playground", def.API.GraphQLPlayground)
	v.SetDefault("database.dsn", def.Database.DSN)
	v.SetDefault("database.max_open_conns", def.Database.MaxOpenConns)
	v.SetDefault("database.min_idle_conns", def.Database.MinIdleConns)
	v.SetDefault("database.conn_max_lifetime", def.Database.ConnMaxLifetimeSeconds)
	v.SetDefault("redis.url", def.Redis.URL)
	v.SetDefault("logging.level", def.Logging.Level)
	v.SetDefault("logging.format", def.Logging.Format)
	return nil
}
