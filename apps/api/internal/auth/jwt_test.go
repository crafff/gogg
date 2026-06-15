package auth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

const testSecret = "0123456789abcdef0123456789abcdef" // 32 bytes

func TestNewIssuer_RejectsShortSecret(t *testing.T) {
	if _, err := NewIssuer("short", time.Minute, time.Hour, "test"); err == nil {
		t.Fatal("expected error for short secret")
	}
}

func TestNewIssuer_RefreshMustOutliveAccess(t *testing.T) {
	if _, err := NewIssuer(testSecret, time.Hour, time.Minute, "test"); err == nil {
		t.Fatal("expected error when refresh ttl <= access ttl")
	}
}

func TestIssueAndParseAccess_RoundTrip(t *testing.T) {
	iss, err := NewIssuer(testSecret, 15*time.Minute, 30*24*time.Hour, "gogg.test")
	if err != nil {
		t.Fatal(err)
	}
	uid := uuid.New()

	token, exp, err := iss.IssueAccess(uid)
	if err != nil {
		t.Fatal(err)
	}
	if time.Until(exp) > 16*time.Minute || time.Until(exp) < 14*time.Minute {
		t.Fatalf("expiry out of expected window: %v", exp)
	}

	claims, err := iss.Parse(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != uid {
		t.Fatalf("user id round-trip: got %v want %v", claims.UserID, uid)
	}
	if claims.Issuer != "gogg.test" {
		t.Fatalf("issuer: got %q", claims.Issuer)
	}
}

func TestParse_RejectsTamperedSignature(t *testing.T) {
	iss, _ := NewIssuer(testSecret, time.Hour, 2*time.Hour, "gogg.test")
	tok, _, _ := iss.IssueAccess(uuid.New())

	tampered := tok[:len(tok)-2] + "ab"
	_, err := iss.Parse(tampered)
	if err == nil {
		t.Fatal("expected error for tampered token")
	}
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("want ErrInvalidToken, got %v", err)
	}
}

func TestParse_RejectsWrongIssuer(t *testing.T) {
	signer, _ := NewIssuer(testSecret, time.Hour, 2*time.Hour, "issuer-a")
	verifier, _ := NewIssuer(testSecret, time.Hour, 2*time.Hour, "issuer-b")

	tok, _, _ := signer.IssueAccess(uuid.New())
	if _, err := verifier.Parse(tok); err == nil {
		t.Fatal("expected issuer-mismatch rejection")
	}
}

func TestNewOpaqueToken_LengthAndUnique(t *testing.T) {
	a, err := NewOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("two opaque tokens collided")
	}
	if !strings.ContainsAny(a, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_") {
		t.Fatalf("opaque token not url-safe base64: %q", a)
	}
	// 32 bytes → 43 chars no padding.
	if len(a) != 43 {
		t.Fatalf("len: got %d want 43", len(a))
	}
}

func TestHashRefreshToken_Stable(t *testing.T) {
	h1 := HashRefreshToken("hello")
	h2 := HashRefreshToken("hello")
	h3 := HashRefreshToken("world")
	if string(h1) != string(h2) {
		t.Fatal("hash not deterministic")
	}
	if string(h1) == string(h3) {
		t.Fatal("different inputs collided")
	}
	if len(h1) != 32 {
		t.Fatalf("sha-256 length: got %d want 32", len(h1))
	}
}
