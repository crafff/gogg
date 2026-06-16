// Package catalog exposes the small read-only "catalog" surface of
// the API — versions and regions for which we have ingested data.
// These power the dropdowns in the rankings UI's filter panel.
//
// Catalog data is a few rows that change once per patch (versions)
// or roughly never (regions); responses are tiny and cache nicely.
// The cache layer wraps this service in a later Phase B milestone.
package catalog

import (
	"context"
	"fmt"
)

// Querier is the narrow subset of sqlc bindings this service needs.
// Defining it here (consumer-side) keeps the service decoupled from
// the generated package and lets tests mock with a hand-rolled type.
// The full sqlcgen.Queries struct satisfies this via duck-typing.
type Querier interface {
	ListVersionsWithData(ctx context.Context) ([]string, error)
	ListRegionsWithData(ctx context.Context) ([]string, error)
}

// Service serves the catalog endpoints. Construct with New() and pass
// the result to the transport layer; do not zero-value it.
type Service struct {
	q Querier
}

// New returns a Service bound to the given Querier.
func New(q Querier) *Service {
	return &Service{q: q}
}

// ListVersionsWithData returns the distinct match-processing versions
// with completed matches, newest first. Always returns a non-nil slice
// (possibly empty) so JSON encoding produces [] not null — the legacy
// /api/versions handler does the same coalescing, and we preserve
// byte-equality for parity testing.
func (s *Service) ListVersionsWithData(ctx context.Context) ([]string, error) {
	out, err := s.q.ListVersionsWithData(ctx)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	if out == nil {
		out = []string{}
	}
	return out, nil
}

// ListRegionsWithData returns the distinct regions with completed
// matches, alphabetically. Same nil-to-empty coalescing as above.
func (s *Service) ListRegionsWithData(ctx context.Context) ([]string, error) {
	out, err := s.q.ListRegionsWithData(ctx)
	if err != nil {
		return nil, fmt.Errorf("list regions: %w", err)
	}
	if out == nil {
		out = []string{}
	}
	return out, nil
}
