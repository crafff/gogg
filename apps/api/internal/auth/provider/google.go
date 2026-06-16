package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Google is the Provider for Google OAuth2. Construct via NewGoogle.
type Google struct {
	cfg *oauth2.Config
}

// NewGoogle wires a Google provider. The two minimum scopes are
// openid + email + profile — enough for UserInfo. We don't request
// offline_access in V1 since gogg doesn't talk to Google APIs on the
// user's behalf after the initial link.
func NewGoogle(clientID, clientSecret, redirectURL string) *Google {
	return &Google{cfg: &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes: []string{
			"openid",
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}}
}

// Name implements Provider.
func (g *Google) Name() string { return "google" }

// AuthCodeURL implements Provider.
func (g *Google) AuthCodeURL(state string) string {
	return g.cfg.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

// googleUser mirrors the v3 userinfo response. The Sub field is the
// stable Google account id (a numeric string). Picture is a CDN URL
// that may rotate — we cache it but treat it as best-effort.
type googleUser struct {
	Sub     string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

// Exchange implements Provider.
func (g *Google) Exchange(ctx context.Context, code string) (UserInfo, error) {
	tok, err := g.cfg.Exchange(ctx, code)
	if err != nil {
		return UserInfo{}, fmt.Errorf("google token exchange: %w", err)
	}
	client := g.cfg.Client(ctx, tok)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v3/userinfo", nil)
	if err != nil {
		return UserInfo{}, fmt.Errorf("google userinfo request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return UserInfo{}, fmt.Errorf("google userinfo fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return UserInfo{}, fmt.Errorf("google userinfo: status %d", resp.StatusCode)
	}

	var u googleUser
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return UserInfo{}, fmt.Errorf("google userinfo decode: %w", err)
	}
	if u.Sub == "" {
		return UserInfo{}, ErrUserInfoIncomplete
	}
	return UserInfo{
		Subject:  u.Sub,
		Email:    u.Email,
		Username: u.Name,
		Avatar:   u.Picture,
	}, nil
}
