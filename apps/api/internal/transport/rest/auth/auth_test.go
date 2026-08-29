package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	usersvc "github.com/crafff/gogg/apps/api/internal/service/user"
)

type fakeService struct {
	beginReturnTo string
	beginBinding  string
	complete      usersvc.BrowserSession
	returnTo      string
	logoutToken   string
	logoutErr     error
}

type fakeLimiter struct {
	allowed    bool
	retryAfter time.Duration
	err        error
	key        string
	limit      int
	window     time.Duration
}

func (f *fakeLimiter) AllowFixedWindow(_ context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	f.key, f.limit, f.window = key, limit, window
	return f.allowed, f.retryAfter, f.err
}

func (f *fakeService) BeginOAuth(_ context.Context, _, returnTo, binding string) (usersvc.OAuthStart, error) {
	f.beginReturnTo = returnTo
	f.beginBinding = binding
	return usersvc.OAuthStart{URL: "https://accounts.example/authorize"}, nil
}

func (f *fakeService) CompleteOAuth(_ context.Context, _, _, _, _, _ string, _ netip.Addr) (usersvc.BrowserSession, string, error) {
	return f.complete, f.returnTo, nil
}

func (f *fakeService) LogoutBrowserSession(_ context.Context, token string) error {
	f.logoutToken = token
	return f.logoutErr
}

func TestStartBindsBrowserAndRejectsExternalReturnTo(t *testing.T) {
	svc := &fakeService{}
	req := httptest.NewRequest(http.MethodGet, "/oauth/start/google?returnTo=https://evil.example", nil)
	rec := httptest.NewRecorder()
	Routes(svc, Config{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	if got := rec.Header().Get("Location"); got != "https://accounts.example/authorize" {
		t.Errorf("Location = %q", got)
	}
	if svc.beginReturnTo != "/me" {
		t.Errorf("returnTo = %q, want /me", svc.beginReturnTo)
	}
	if svc.beginBinding == "" {
		t.Fatal("browser binding was empty")
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "gogg_oauth_browser" || !cookies[0].HttpOnly || cookies[0].Path != "/oauth" {
		t.Fatalf("binding cookies = %#v", cookies)
	}
}

func TestStartRateLimitStopsAttemptBeforeDatabaseWrite(t *testing.T) {
	svc := &fakeService{}
	limiter := &fakeLimiter{allowed: false, retryAfter: 90 * time.Second}
	req := httptest.NewRequest(http.MethodGet, "/oauth/start/google", nil)
	rec := httptest.NewRecorder()
	Routes(svc, Config{Limiter: limiter}).ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	if got := rec.Header().Get("Retry-After"); got != "90" {
		t.Errorf("Retry-After = %q, want 90", got)
	}
	if !strings.HasPrefix(limiter.key, "oauth-start:ip:") || limiter.limit != 10 || limiter.window != 10*time.Minute {
		t.Fatalf("limiter call = key %q, limit %d, window %v", limiter.key, limiter.limit, limiter.window)
	}
	if svc.beginBinding != "" {
		t.Fatal("rate-limited request reached BeginOAuth")
	}
}

func TestCallbackSetsOpaqueSessionCookieAndLocalRedirect(t *testing.T) {
	svc := &fakeService{
		complete: usersvc.BrowserSession{
			Token: "opaque-session-secret", ExpiresAt: time.Now().Add(time.Hour), UserID: uuid.New(),
		},
		returnTo: "/summoner/NA1/Player/Tag?tab=ranked",
	}
	req := httptest.NewRequest(http.MethodGet, "/oauth/callback/google?code=code&state=state", nil)
	req.AddCookie(&http.Cookie{Name: "gogg_oauth_browser", Value: "binding"})
	rec := httptest.NewRecorder()
	Routes(svc, Config{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("Referrer-Policy = %q, want no-referrer", got)
	}
	if got := rec.Header().Get("Location"); got != svc.returnTo {
		t.Errorf("Location = %q, want %q", got, svc.returnTo)
	}
	var session *http.Cookie
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "gogg_session" {
			session = cookie
		}
	}
	if session == nil || session.Value != "opaque-session-secret" || !session.HttpOnly || session.SameSite != http.SameSiteStrictMode {
		t.Fatalf("session cookie = %#v", session)
	}
	if strings.Contains(rec.Body.String(), "opaque-session-secret") {
		t.Fatal("response body exposed the session token")
	}
}

func TestLogoutRequiresCSRFAndOnlyClearsAfterRevocation(t *testing.T) {
	for _, tc := range []struct {
		name       string
		header     string
		logoutErr  error
		wantStatus int
		wantClear  bool
	}{
		{name: "missing csrf", wantStatus: http.StatusForbidden},
		{name: "revoke failure", header: "1", logoutErr: errors.New("database down"), wantStatus: http.StatusServiceUnavailable},
		{name: "success", header: "1", wantStatus: http.StatusNoContent, wantClear: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeService{logoutErr: tc.logoutErr}
			req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
			req.AddCookie(&http.Cookie{Name: "gogg_session", Value: "session-secret"})
			if tc.header != "" {
				req.Header.Set(CSRFHeader, tc.header)
			}
			rec := httptest.NewRecorder()
			Routes(svc, Config{}).ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			cleared := false
			for _, cookie := range rec.Result().Cookies() {
				cleared = cleared || (cookie.Name == "gogg_session" && cookie.MaxAge < 0)
			}
			if cleared != tc.wantClear {
				t.Errorf("cookie cleared = %v, want %v", cleared, tc.wantClear)
			}
			if tc.header == "1" && svc.logoutToken != "session-secret" {
				t.Errorf("logout token = %q", svc.logoutToken)
			}
		})
	}
}

func TestSafeReturnTo(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want string
	}{
		{raw: "/me?tab=accounts", want: "/me?tab=accounts"},
		{raw: "https://evil.example", want: "/me"},
		{raw: "//evil.example", want: "/me"},
		{raw: `/\\evil.example`, want: "/me"},
	} {
		if got := safeReturnTo(tc.raw, "/me"); got != tc.want {
			t.Errorf("safeReturnTo(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}
