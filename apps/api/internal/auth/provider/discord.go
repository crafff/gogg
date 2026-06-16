package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
)

// discordEndpoint is the OAuth2 endpoint per
// https://discord.com/developers/docs/topics/oauth2. Inlined because
// the oauth2 stdlib package doesn't ship a Discord endpoint constant.
var discordEndpoint = oauth2.Endpoint{
	AuthURL:  "https://discord.com/oauth2/authorize",
	TokenURL: "https://discord.com/api/oauth2/token",
}

// Discord is the Provider for Discord OAuth2. Construct via NewDiscord.
type Discord struct {
	cfg *oauth2.Config
}

// NewDiscord wires a Discord provider. Scopes default to identify +
// email, which is the minimum to populate UserInfo.
func NewDiscord(clientID, clientSecret, redirectURL string) *Discord {
	return &Discord{cfg: &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"identify", "email"},
		Endpoint:     discordEndpoint,
	}}
}

// Name implements Provider.
func (d *Discord) Name() string { return "discord" }

// AuthCodeURL implements Provider.
func (d *Discord) AuthCodeURL(state string) string {
	return d.cfg.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

// discordUser mirrors the subset of /users/@me we read. Discord
// returns more fields (locale, banner, premium tier) but they don't
// belong on the gogg user record.
type discordUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Avatar   string `json:"avatar"`
}

// Exchange implements Provider.
func (d *Discord) Exchange(ctx context.Context, code string) (UserInfo, error) {
	tok, err := d.cfg.Exchange(ctx, code)
	if err != nil {
		return UserInfo{}, fmt.Errorf("discord token exchange: %w", err)
	}
	client := d.cfg.Client(ctx, tok)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://discord.com/api/users/@me", nil)
	if err != nil {
		return UserInfo{}, fmt.Errorf("discord userinfo request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return UserInfo{}, fmt.Errorf("discord userinfo fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return UserInfo{}, fmt.Errorf("discord userinfo: status %d", resp.StatusCode)
	}

	var u discordUser
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return UserInfo{}, fmt.Errorf("discord userinfo decode: %w", err)
	}
	if u.ID == "" {
		return UserInfo{}, ErrUserInfoIncomplete
	}

	info := UserInfo{Subject: u.ID, Email: u.Email, Username: u.Username}
	if u.Avatar != "" {
		info.Avatar = fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png", u.ID, u.Avatar)
	}
	return info, nil
}
