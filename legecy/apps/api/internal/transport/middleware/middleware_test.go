package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestID_generatesWhenAbsent(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()

	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := RequestIDFromContext(r.Context())
		if id == "" {
			t.Fatal("expected request id in context")
		}
		_, _ = w.Write([]byte(id))
	}))
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get(HeaderRequestID); got == "" {
		t.Errorf("response should include %s header", HeaderRequestID)
	}
	if got, want := rec.Header().Get(HeaderRequestID), rec.Body.String(); got != want {
		t.Errorf("response header %q != context value %q", got, want)
	}
}

func TestRequestID_passesThroughInbound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(HeaderRequestID, "from-upstream")
	rec := httptest.NewRecorder()

	handler := RequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if got := RequestIDFromContext(r.Context()); got != "from-upstream" {
			t.Errorf("context request_id = %q, want from-upstream", got)
		}
	}))
	handler.ServeHTTP(rec, req)
	if rec.Header().Get(HeaderRequestID) != "from-upstream" {
		t.Errorf("response header lost the upstream id")
	}
}

func TestLogger_attachesContextLoggerAndEmitsEntry(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	req := httptest.NewRequest(http.MethodPost, "/y", nil)
	rec := httptest.NewRecorder()
	handler := RequestID(Logger(base)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		LoggerFromContext(r.Context()).Info("inside_handler")
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("brew"))
	})))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d", rec.Code)
	}

	// Two log lines: one from inside the handler, one from the
	// middleware itself. The middleware line carries status=418.
	var found bool
	for _, line := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		var rec map[string]any
		if err := json.Unmarshal(line, &rec); err != nil {
			t.Fatalf("log line %q: %v", string(line), err)
		}
		if rec["msg"] == "http_request" {
			found = true
			if status, _ := rec["status"].(float64); int(status) != http.StatusTeapot {
				t.Errorf("status field = %v, want 418", rec["status"])
			}
		}
	}
	if !found {
		t.Error("never saw the http_request log line")
	}
}

func TestLoggerFromContext_fallbackToDefault(t *testing.T) {
	if LoggerFromContext(context.Background()) == nil {
		t.Error("should never return nil")
	}
}

func TestRecover_returns500_andDoesNotLeakPanicMsg(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(slog.NewJSONHandler(&buf, nil))

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	rec := httptest.NewRecorder()
	stack := RequestID(Logger(base)(Recover(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("super secret reason")
	}))))
	stack.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "super secret reason") {
		t.Error("panic message leaked to client")
	}
	if !strings.Contains(buf.String(), "panic_recovered") {
		t.Error("panic_recovered log entry missing")
	}
}

func TestCORS_echoesAllowedOriginOnly(t *testing.T) {
	mw := CORS([]string{"http://localhost:5173"})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	cases := []struct {
		origin string
		want   string
	}{
		{"http://localhost:5173", "http://localhost:5173"},
		{"http://evil.example.com", ""},
		{"", ""},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tc.want {
			t.Errorf("origin %q: header = %q, want %q", tc.origin, got, tc.want)
		}
	}
}

func TestCORS_preflight_204(t *testing.T) {
	mw := CORS([]string{"http://localhost:5173"})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("preflight should short-circuit before reaching the inner handler")
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("preflight status = %d, want 204", rec.Code)
	}
}
