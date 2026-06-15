package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// NewOpaqueToken returns a 256-bit url-safe random token. Used for the
// refresh token cookie; clients return it verbatim on /auth/refresh
// and the server looks it up by sha-256 hash.
//
// Length: 32 bytes raw → 43 base64url chars (no padding). Cookie size
// fits comfortably under the 4 KiB browser limit even after the JWT
// access token is set alongside it.
func NewOpaqueToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("crypto rand: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
