package provider

import (
	"net/url"
	"testing"

	"golang.org/x/oauth2"
)

func TestGoogleAuthCodeURLUsesStateAndPKCE(t *testing.T) {
	provider := NewGoogle("client-id", "client-secret", "http://localhost:5173/oauth/callback/google")
	verifier := oauth2.GenerateVerifier()

	raw := provider.AuthCodeURL("opaque-state", verifier)
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	query := parsed.Query()
	if got := query.Get("state"); got != "opaque-state" {
		t.Errorf("state = %q, want opaque-state", got)
	}
	if got := query.Get("code_challenge_method"); got != "S256" {
		t.Errorf("code_challenge_method = %q, want S256", got)
	}
	if got, want := query.Get("code_challenge"), oauth2.S256ChallengeFromVerifier(verifier); got != want {
		t.Errorf("code_challenge = %q, want %q", got, want)
	}
	if query.Has("code_verifier") || query.Get("code_challenge") == verifier {
		t.Fatal("authorization URL exposed the PKCE verifier")
	}
}
