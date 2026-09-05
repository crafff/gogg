package config

import (
	"strings"
	"testing"
	"time"
)

func TestDefault_validates(t *testing.T) {
	if err := Default().Validate(); err != nil {
		t.Fatalf("Default() should pass validation, got: %v", err)
	}
}

func TestValidate_catchesProblems(t *testing.T) {
	tests := []struct {
		name string
		mut  func(*Config)
		want string
	}{
		{"port zero", func(c *Config) { c.API.Port = 0 }, "api.port"},
		{"port too high", func(c *Config) { c.API.Port = 70000 }, "api.port"},
		{"empty dsn", func(c *Config) { c.Database.DSN = "" }, "database.dsn"},
		{"bad log level", func(c *Config) { c.Logging.Level = "trace" }, "logging.level"},
		{"bad log format", func(c *Config) { c.Logging.Format = "csv" }, "logging.format"},
		{"zero read timeout", func(c *Config) { c.API.ReadTimeout = 0 }, "api.read_timeout"},
		{"zero browser session ttl", func(c *Config) { c.Auth.RefreshTTL = 0 }, "auth.refresh_ttl"},
		{"short jwt secret", func(c *Config) { c.Auth.JWTSecret = "too-short" }, "at least 32 bytes"},
		{"partial google config", func(c *Config) { c.OAuth.Google.ClientID = "client" }, "configured together"},
		{"insecure remote callback", func(c *Config) {
			c.OAuth.Google = OAuthProviderConfig{
				ClientID: "client", ClientSecret: "secret", RedirectURL: "http://example.com/oauth/callback/google",
			}
		}, "must use https"},
		{"https callback without secure cookie", func(c *Config) {
			c.OAuth.Google = OAuthProviderConfig{
				ClientID: "client", ClientSecret: "secret", RedirectURL: "https://gogg.example/oauth/callback/google",
			}
		}, "cookie_secure"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Default()
			tc.mut(&cfg)
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Validate() should have failed")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q should mention %q", err.Error(), tc.want)
			}
		})
	}
}

func TestValidate_acceptsLocalAndHTTPSOAuthCallbacks(t *testing.T) {
	for _, redirectURL := range []string{
		"http://localhost:5173/oauth/callback/google",
		"http://127.0.0.1:5173/oauth/callback/google",
	} {
		t.Run(redirectURL, func(t *testing.T) {
			cfg := Default()
			cfg.OAuth.Google = OAuthProviderConfig{
				ClientID: "client", ClientSecret: "secret", RedirectURL: redirectURL,
			}
			if err := cfg.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}

	cfg := Default()
	cfg.Auth.CookieSecure = true
	cfg.OAuth.Google = OAuthProviderConfig{
		ClientID: "client", ClientSecret: "secret", RedirectURL: "https://gogg.example/oauth/callback/google",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("secure production callback: Validate() error = %v", err)
	}
}

func TestValidate_aggregatesErrors(t *testing.T) {
	cfg := Default()
	cfg.API.Port = 0
	cfg.Database.DSN = ""
	cfg.Logging.Level = "nope"
	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected aggregated error")
	}
	msg := err.Error()
	for _, want := range []string{"api.port", "database.dsn", "logging.level"} {
		if !strings.Contains(msg, want) {
			t.Errorf("aggregated error %q missing %q", msg, want)
		}
	}
}

func TestLoad_envOverridesDefault(t *testing.T) {
	t.Setenv("GOGG_API_PORT", "9999")
	t.Setenv("GOGG_LOGGING_LEVEL", "debug")
	t.Setenv("APP_CONFIG_PATH", "") // skip file path

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() err = %v", err)
	}
	if cfg.API.Port != 9999 {
		t.Errorf("Port = %d, want 9999", cfg.API.Port)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Level = %q, want debug", cfg.Logging.Level)
	}
	if cfg.API.ReadTimeout != 10*time.Second {
		t.Errorf("ReadTimeout should keep default 10s, got %v", cfg.API.ReadTimeout)
	}
}
