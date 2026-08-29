package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// NewOpaqueToken returns a 256-bit URL-safe random token. Browser sessions,
// OAuth state, and browser-binding secrets use it; persistent storage keeps
// only the SHA-256 digest where lookup is required.
//
// Length: 32 bytes raw → 43 base64url chars (no padding). Cookie size
// fits comfortably under the 4 KiB browser cookie limit.
func NewOpaqueToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("crypto rand: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
