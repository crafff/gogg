package riotapi

import (
	"fmt"
	"testing"
)

func TestIsUnauthorized(t *testing.T) {
	err := fmt.Errorf("fetch league entries: %w", &APIError{
		Kind:       ErrorUnauthorized,
		StatusCode: 403,
		Global:     true,
	})
	if !IsUnauthorized(err) {
		t.Fatal("expected wrapped unauthorized API error to be recognized")
	}
	if IsUnauthorized(&APIError{Kind: ErrorNotFound, StatusCode: 404}) {
		t.Fatal("did not expect not-found API error to be unauthorized")
	}
}
