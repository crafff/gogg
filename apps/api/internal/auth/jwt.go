// Package auth issues and validates the JWT access tokens the API
// uses for stateful endpoints, plus opaque refresh tokens whose hash
// lives in the database.
//
// V1 ships HS256 with a shared secret loaded from SOPS. Phase F's
// production hardening upgrades to RS256 with key rotation so a single
// signing key compromise no longer means every gogg user must
// re-authenticate — see the plan §5 (Phase F). Until then, treat the
// secret like the database password.
//
// Token model:
//
//	access  — 15 min, JWT, carried in the Authorization header
//	refresh — 30 days, opaque random string, stored as sha-256 in
//	          user_refresh_tokens, sent back in an HttpOnly cookie
//
// Access tokens are stateless (no DB lookup needed on each request),
// refresh tokens are stateful so we can revoke them on logout or on
// account compromise without waiting for the access TTL.
package auth

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims is the parsed shape of an access token. Subject is the user
// UUID rendered as a canonical string; the JWT std fields (iat, exp,
// jti) carry validity bounds and the unique id used to revoke this
// specific token.
type Claims struct {
	UserID uuid.UUID `json:"-"`
	jwt.RegisteredClaims
}

// Issuer signs and parses access tokens. Construct with NewIssuer once
// at startup and reuse — internal state is just the secret and the
// configured TTLs.
type Issuer struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	issuer     string
}

// NewIssuer returns a configured Issuer. accessTTL and refreshTTL are
// the validity windows for the two token classes. issuer is the `iss`
// claim — set this to the public hostname (e.g. "api.gogg.gg") so
// clients can pin per-environment.
func NewIssuer(secret string, accessTTL, refreshTTL time.Duration, issuer string) (*Issuer, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("auth: jwt secret must be at least 32 bytes (got %d)", len(secret))
	}
	if accessTTL <= 0 || refreshTTL <= 0 {
		return nil, fmt.Errorf("auth: token ttls must be > 0")
	}
	if refreshTTL <= accessTTL {
		return nil, fmt.Errorf("auth: refresh ttl must outlive access ttl")
	}
	return &Issuer{
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		issuer:     strings.TrimSpace(issuer),
	}, nil
}

// AccessTTL is read by the REST layer so the response can echo back
// the expiry without each handler re-deriving it.
func (i *Issuer) AccessTTL() time.Duration { return i.accessTTL }

// RefreshTTL mirrors AccessTTL — exposed so the refresh-token table
// insert can set expires_at = now() + RefreshTTL.
func (i *Issuer) RefreshTTL() time.Duration { return i.refreshTTL }

// IssueAccess signs an access token for the given user. jti is a fresh
// uuid so any individual access token can be revoked via a deny list
// once that surface exists.
func (i *Issuer) IssueAccess(userID uuid.UUID) (token string, expiresAt time.Time, err error) {
	now := time.Now().UTC()
	expiresAt = now.Add(i.accessTTL)
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    i.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        uuid.NewString(),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString(i.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return signed, expiresAt, nil
}

// ErrInvalidToken is returned by Parse when the token failed any of
// the standard validations (signature, expiry, not-before, issuer).
// Callers compare with errors.Is, not string match.
var ErrInvalidToken = errors.New("auth: invalid token")

// Parse validates the signature, expiry, and issuer, returning the
// extracted claims. The userID is parsed out of the subject and
// returned in the Claims struct (jwt.RegisteredClaims keeps it as a
// string).
func (i *Issuer) Parse(token string) (*Claims, error) {
	c := &Claims{}
	parsed, err := jwt.ParseWithClaims(token, c, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %s", t.Header["alg"])
		}
		return i.secret, nil
	},
		jwt.WithIssuer(i.issuer),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if !parsed.Valid {
		return nil, ErrInvalidToken
	}
	uid, err := uuid.Parse(c.Subject)
	if err != nil {
		return nil, fmt.Errorf("%w: subject not a uuid: %w", ErrInvalidToken, err)
	}
	c.UserID = uid
	return c, nil
}

// HashRefreshToken returns the sha-256 of the opaque refresh token
// string. Callers store this in user_refresh_tokens.token_hash;
// the cleartext only travels in the cookie.
//
// sha-256 with no salt is acceptable here because the refresh tokens
// are 256 bits of crypto/rand — there's no rainbow-table or password
// cracking threat. The hash is just so a DB read doesn't yield
// usable tokens.
func HashRefreshToken(s string) []byte {
	sum := sha256.Sum256([]byte(s))
	return sum[:]
}
