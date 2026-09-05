// Package config loads and validates the gogg-api configuration.
//
// Layering, highest precedence last:
//
//  1. Defaults baked into Default().
//  2. YAML file pointed at by APP_CONFIG_PATH (default: ./config/dev.yaml).
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
	"net"
	"net/url"
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
	Temporal TemporalConfig `mapstructure:"temporal"`
	Summoner SummonerConfig `mapstructure:"summoner"`
	TFT      TFTConfig      `mapstructure:"tft"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Auth     AuthConfig     `mapstructure:"auth"`
	OAuth    OAuthConfig    `mapstructure:"oauth"`
	Assets   AssetConfig    `mapstructure:"assets"`
}

type TemporalConfig struct {
	HostPort         string            `mapstructure:"host_port"`
	Namespace        string            `mapstructure:"namespace"`
	RegionTaskQueues map[string]string `mapstructure:"region_task_queues"`
}

type SummonerConfig struct {
	Freshness     time.Duration `mapstructure:"freshness"`
	IPLimit       int           `mapstructure:"ip_limit"`
	IPLimitWindow time.Duration `mapstructure:"ip_limit_window"`
}

type TFTConfig struct {
	PlayerFreshness     time.Duration `mapstructure:"player_freshness"`
	PlayerIPLimit       int           `mapstructure:"player_ip_limit"`
	PlayerIPLimitWindow time.Duration `mapstructure:"player_ip_limit_window"`
}

type AssetConfig struct {
	Root string `mapstructure:"root"`
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

// AuthConfig controls JWT issuance + the cookie surface. JWTSecret is
// the HS256 signing key (Phase F upgrades to RS256). Issuer is the
// JWT `iss` claim — clients pin per-env. CookieSecure must be true in
// any deployment served over https; dev / compose runs leave it false.
type AuthConfig struct {
	JWTSecret    string        `mapstructure:"jwt_secret"`
	Issuer       string        `mapstructure:"issuer"`
	AccessTTL    time.Duration `mapstructure:"access_ttl"`
	RefreshTTL   time.Duration `mapstructure:"refresh_ttl"`
	CookieDomain string        `mapstructure:"cookie_domain"`
	CookieSecure bool          `mapstructure:"cookie_secure"`
}

// OAuthConfig wires the supported providers. Empty client_id means
// "do not register this provider"; the callback at
// /oauth/start/{provider} returns 404 for any unregistered name.
// Riot RSO lands in this struct under a build tag once approval lands.
type OAuthConfig struct {
	Discord OAuthProviderConfig `mapstructure:"discord"`
	Google  OAuthProviderConfig `mapstructure:"google"`
}

// OAuthProviderConfig is the per-provider tuple. RedirectURL must be
// the absolute URL of the callback (e.g.
// "https://api.gogg.gg/oauth/callback/discord") — it has to match the
// value registered in the provider's developer console exactly.
type OAuthProviderConfig struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	RedirectURL  string `mapstructure:"redirect_url"`
}

// Default returns a Config populated with safe defaults for local dev.
// Production values are expected to come from decrypted config + env vars.
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
		Temporal: TemporalConfig{
			HostPort:  "localhost:7233",
			Namespace: "default",
			RegionTaskQueues: map[string]string{
				"KR": "crawl-kr", "NA1": "crawl-na1",
			},
		},
		Summoner: SummonerConfig{
			Freshness: 5 * time.Minute, IPLimit: 5, IPLimitWindow: 10 * time.Minute,
		},
		TFT:    TFTConfig{PlayerFreshness: 10 * time.Minute, PlayerIPLimit: 5, PlayerIPLimitWindow: 10 * time.Minute},
		Assets: AssetConfig{Root: "data/game-assets"},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Auth: AuthConfig{
			Issuer:       "gogg.local",
			AccessTTL:    15 * time.Minute,
			RefreshTTL:   30 * 24 * time.Hour,
			CookieDomain: "",
			CookieSecure: false,
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
	// to exist; the default ./config/dev.yaml is read best-effort.
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
	if strings.TrimSpace(c.Temporal.HostPort) == "" || strings.TrimSpace(c.Temporal.Namespace) == "" {
		errs = append(errs, fmt.Errorf("temporal host_port and namespace are required"))
	}
	for _, region := range []string{"KR", "NA1"} {
		if strings.TrimSpace(c.Temporal.RegionTaskQueues[region]) == "" {
			errs = append(errs, fmt.Errorf("temporal.region_task_queues.%s is required", region))
		}
	}
	if c.Summoner.Freshness <= 0 || c.Summoner.IPLimit <= 0 || c.Summoner.IPLimitWindow <= 0 {
		errs = append(errs, fmt.Errorf("summoner freshness and public rate limits must be > 0"))
	}
	if c.TFT.PlayerFreshness <= 0 || c.TFT.PlayerIPLimit <= 0 || c.TFT.PlayerIPLimitWindow <= 0 {
		errs = append(errs, fmt.Errorf("TFT player freshness and public rate limits must be > 0"))
	}
	if c.Auth.AccessTTL <= 0 {
		errs = append(errs, fmt.Errorf("auth.access_ttl must be > 0"))
	}
	if c.Auth.RefreshTTL <= 0 {
		errs = append(errs, fmt.Errorf("auth.refresh_ttl must be > 0"))
	}
	if c.Auth.JWTSecret != "" {
		if len(c.Auth.JWTSecret) < 32 {
			errs = append(errs, fmt.Errorf("auth.jwt_secret must be at least 32 bytes when enabled"))
		}
		if c.Auth.RefreshTTL <= c.Auth.AccessTTL {
			errs = append(errs, fmt.Errorf("auth.refresh_ttl must outlive auth.access_ttl when JWT is enabled"))
		}
	}
	for name, provider := range map[string]OAuthProviderConfig{
		"discord": c.OAuth.Discord,
		"google":  c.OAuth.Google,
	} {
		if err := validateOAuthProvider(name, provider); err != nil {
			errs = append(errs, err)
		}
	}
	if googleURL, ok := configuredOAuthRedirect(c.OAuth.Google); ok {
		if (googleURL.Scheme == "https") != c.Auth.CookieSecure {
			errs = append(errs, fmt.Errorf("auth.cookie_secure must match the configured Google redirect URL scheme"))
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
	return errors.Join(errs...)
}

func validateOAuthProvider(name string, provider OAuthProviderConfig) error {
	clientID := strings.TrimSpace(provider.ClientID)
	clientSecret := strings.TrimSpace(provider.ClientSecret)
	redirectURL := strings.TrimSpace(provider.RedirectURL)
	configured := clientID != "" || clientSecret != "" || redirectURL != ""
	if !configured {
		return nil
	}
	if clientID == "" || clientSecret == "" || redirectURL == "" {
		return fmt.Errorf("oauth.%s client_id, client_secret, and redirect_url must be configured together", name)
	}

	parsed, err := url.Parse(redirectURL)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || parsed.Path == "" {
		return fmt.Errorf("oauth.%s.redirect_url must be an absolute callback URL", name)
	}
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname())) {
		return fmt.Errorf("oauth.%s.redirect_url must use https outside local development", name)
	}
	return nil
}

func configuredOAuthRedirect(provider OAuthProviderConfig) (*url.URL, bool) {
	if strings.TrimSpace(provider.ClientID) == "" || strings.TrimSpace(provider.ClientSecret) == "" || strings.TrimSpace(provider.RedirectURL) == "" {
		return nil, false
	}
	parsed, err := url.Parse(strings.TrimSpace(provider.RedirectURL))
	return parsed, err == nil && parsed.IsAbs() && parsed.Host != ""
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
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
	v.SetDefault("temporal.host_port", def.Temporal.HostPort)
	v.SetDefault("temporal.namespace", def.Temporal.Namespace)
	v.SetDefault("temporal.region_task_queues", def.Temporal.RegionTaskQueues)
	v.SetDefault("summoner.freshness", def.Summoner.Freshness)
	v.SetDefault("summoner.ip_limit", def.Summoner.IPLimit)
	v.SetDefault("summoner.ip_limit_window", def.Summoner.IPLimitWindow)
	v.SetDefault("tft.player_freshness", def.TFT.PlayerFreshness)
	v.SetDefault("tft.player_ip_limit", def.TFT.PlayerIPLimit)
	v.SetDefault("tft.player_ip_limit_window", def.TFT.PlayerIPLimitWindow)
	v.SetDefault("logging.level", def.Logging.Level)
	v.SetDefault("logging.format", def.Logging.Format)
	v.SetDefault("auth.jwt_secret", "")
	v.SetDefault("auth.issuer", def.Auth.Issuer)
	v.SetDefault("auth.access_ttl", def.Auth.AccessTTL)
	v.SetDefault("auth.refresh_ttl", def.Auth.RefreshTTL)
	v.SetDefault("auth.cookie_domain", def.Auth.CookieDomain)
	v.SetDefault("auth.cookie_secure", def.Auth.CookieSecure)
	// SetDefault seeds the keyspace so AutomaticEnv finds them. We
	// don't ship default credentials — all values are blank, and the
	// caller / sops file fills them in.
	v.SetDefault("oauth.discord.client_id", "")
	v.SetDefault("oauth.discord.client_secret", "")
	v.SetDefault("oauth.discord.redirect_url", "")
	v.SetDefault("oauth.google.client_id", "")
	v.SetDefault("oauth.google.client_secret", "")
	v.SetDefault("oauth.google.redirect_url", "")
	v.SetDefault("assets.root", def.Assets.Root)
	return nil
}
