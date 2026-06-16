// Package provider abstracts the OAuth providers gogg supports.
// Each implementation embeds an *oauth2.Config and adds a single
// "fetch the userinfo" call.
//
// V1 ships Discord and Google; Riot RSO lands behind a build tag
// once Riot approves the application (per ADR-0001 and the plan §1.5).
// Providers are registered with the REST callback handler at boot;
// any subset can be wired by leaving the config blank.
package provider

import (
	"context"
	"errors"
)

// UserInfo is the normalised view the callback handler stores on the
// user record. Subject is the provider's stable id (Discord uses
// snowflake strings, Google uses opaque numeric strings); Email and
// Username are best-effort labels we cache for display.
type UserInfo struct {
	Subject  string
	Email    string
	Username string
	Avatar   string
}

// Provider is the small surface the callback handler uses. Keeping the
// interface narrow means new providers (Riot RSO, GitHub, …) plug in
// by adding one file in this package and one config knob.
type Provider interface {
	// Name returns the canonical provider slug ("discord", "google").
	// It's used as the column value in user_oauth_identities.provider
	// and as the path segment in /oauth/callback/{provider}.
	Name() string

	// AuthCodeURL returns the URL the browser should be redirected to
	// for the consent step. `state` is an opaque CSRF token the caller
	// generates per attempt and verifies on callback.
	AuthCodeURL(state string) string

	// Exchange turns an authorisation code into a UserInfo. Wraps the
	// oauth2.Config.Exchange + GET /userinfo round trip so callers
	// don't deal with two HTTP clients.
	Exchange(ctx context.Context, code string) (UserInfo, error)
}

// ErrUserInfoIncomplete is returned by an Exchange that succeeded at
// the token step but came back without a Subject. The callback handler
// surfaces this to the user as "could not link account, please retry"
// rather than panicking on the empty id.
var ErrUserInfoIncomplete = errors.New("oauth: provider returned empty subject")
