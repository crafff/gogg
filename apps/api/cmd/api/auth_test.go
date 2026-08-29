package main

import (
	"io"
	"log/slog"
	"testing"

	"github.com/crafff/gogg/apps/api/internal/config"
)

func TestConfiguredProvidersEnablesGoogleOnly(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	providers := configuredProviders(config.OAuthConfig{
		Google: config.OAuthProviderConfig{
			ClientID: "google-client", ClientSecret: "google-secret", RedirectURL: "http://localhost:5173/oauth/callback/google",
		},
		Discord: config.OAuthProviderConfig{
			ClientID: "discord-client", ClientSecret: "discord-secret", RedirectURL: "http://localhost:5173/oauth/callback/discord",
		},
	}, logger)

	if len(providers) != 1 || providers[0].Name() != "google" {
		t.Fatalf("providers = %#v, want google only", providers)
	}
}

func TestConfiguredProvidersSkipsIncompleteGoogle(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	providers := configuredProviders(config.OAuthConfig{
		Google: config.OAuthProviderConfig{ClientID: "client-only"},
	}, logger)
	if len(providers) != 0 {
		t.Fatalf("providers = %#v, want none", providers)
	}
}
