package v1

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeCatalog struct {
	versions []string
	regions  []string
	err      error
}

func (f fakeCatalog) ListVersionsWithData(_ context.Context) ([]string, error) {
	return f.versions, f.err
}
func (f fakeCatalog) ListRegionsWithData(_ context.Context) ([]string, error) {
	return f.regions, f.err
}

func TestVersions_happyPath(t *testing.T) {
	r := Routes(fakeCatalog{versions: []string{"15.1.1", "15.1"}}, nil)
	req := httptest.NewRequest(http.MethodGet, "/versions", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	// Byte-equal-ish parity check vs legacy: the JSON shape must
	// be {"versions":["..."]} in the same field order. Decoded
	// equality is sufficient; the legacy server uses json.Encode
	// with no special ordering hooks.
	var body map[string][]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	got, ok := body["versions"]
	if !ok {
		t.Fatal("missing 'versions' field")
	}
	if len(got) != 2 || got[0] != "15.1.1" || got[1] != "15.1" {
		t.Errorf("versions = %v", got)
	}
}

func TestVersions_emptyIsArrayNotNull(t *testing.T) {
	r := Routes(fakeCatalog{versions: nil}, nil)
	req := httptest.NewRequest(http.MethodGet, "/versions", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Critical for parity: legacy returns "[]" not "null" when there
	// are no completed matches yet. Verify the JSON literal.
	if !strings.Contains(rec.Body.String(), `"versions":[]`) {
		t.Errorf("body should contain \"versions\":[]; got %s", rec.Body.String())
	}
}

func TestVersions_errorIsSanitized(t *testing.T) {
	r := Routes(fakeCatalog{err: errors.New("sensitive db detail")}, nil)
	req := httptest.NewRequest(http.MethodGet, "/versions", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "sensitive db detail") {
		t.Errorf("error body leaked the internal detail: %s", rec.Body.String())
	}
}

func TestRegions_happyPath(t *testing.T) {
	r := Routes(fakeCatalog{regions: []string{"KR", "NA1"}}, nil)
	req := httptest.NewRequest(http.MethodGet, "/regions", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body map[string][]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	got, ok := body["regions"]
	if !ok {
		t.Fatal("missing 'regions' field")
	}
	if len(got) != 2 || got[0] != "KR" || got[1] != "NA1" {
		t.Errorf("regions = %v", got)
	}
}

func TestRankings_invalidFiltersReturn400(t *testing.T) {
	tests := []string{
		"queueId=not-a-number",
		"queueId=10000",
		"minGames=0",
		"positionThreshold=101",
		"version=sixteen",
		"region=EUW1",
		"tier=diamond",
		"position=CENTER",
	}
	for _, query := range tests {
		t.Run(query, func(t *testing.T) {
			r := Routes(fakeCatalog{}, nil)
			req := httptest.NewRequest(http.MethodGet, "/rankings/champions?"+query, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRankings_oversizedQueryReturns414(t *testing.T) {
	r := Routes(fakeCatalog{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/rankings/champions?version="+strings.Repeat("1", 4097), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestURITooLong {
		t.Fatalf("status = %d, want 414", rec.Code)
	}
}
