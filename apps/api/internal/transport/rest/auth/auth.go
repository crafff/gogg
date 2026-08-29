// Package auth exposes the browser redirect endpoints for Google login and the
// application-session logout endpoint. OAuth and application tokens never
// enter the SPA; the browser only receives an opaque HttpOnly session cookie.
package auth

import (
	"context"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	appauth "github.com/crafff/gogg/apps/api/internal/auth"
	usersvc "github.com/crafff/gogg/apps/api/internal/service/user"
	"github.com/crafff/gogg/apps/api/internal/transport/middleware"
)

const CSRFHeader = "X-GOGG-CSRF"

const (
	maxOAuthCodeLength    = 4096
	maxOAuthStateLength   = 512
	maxBindingTokenLength = 512
	maxReturnToLength     = 2048
)

type Service interface {
	BeginOAuth(context.Context, string, string, string) (usersvc.OAuthStart, error)
	CompleteOAuth(context.Context, string, string, string, string, string, netip.Addr) (usersvc.BrowserSession, string, error)
	LogoutBrowserSession(context.Context, string) error
}

type FixedWindowLimiter interface {
	AllowFixedWindow(context.Context, string, int, time.Duration) (bool, time.Duration, error)
}

type Config struct {
	CookieSecure     bool
	DefaultReturnTo  string
	FailureRedirect  string
	Limiter          FixedWindowLimiter
	OAuthStartLimit  int
	OAuthStartWindow time.Duration
}

func Routes(svc Service, cfg Config) chi.Router {
	if cfg.DefaultReturnTo == "" {
		cfg.DefaultReturnTo = "/me"
	}
	if cfg.FailureRedirect == "" {
		cfg.FailureRedirect = "/login"
	}
	if cfg.OAuthStartLimit <= 0 {
		cfg.OAuthStartLimit = 10
	}
	if cfg.OAuthStartWindow <= 0 {
		cfg.OAuthStartWindow = 10 * time.Minute
	}
	r := chi.NewRouter()
	h := &handler{svc: svc, cfg: cfg}
	r.Get("/oauth/start/{provider}", h.start)
	r.Get("/oauth/callback/{provider}", h.callback)
	r.Post("/auth/logout", h.logout)
	return r
}

type handler struct {
	svc Service
	cfg Config
}

func SessionCookieName(secure bool) string {
	if secure {
		return "__Host-gogg_session"
	}
	return "gogg_session"
}

func browserBindingCookieName(secure bool) string {
	if secure {
		return "__Secure-gogg_oauth_browser"
	}
	return "gogg_oauth_browser"
}

func (h *handler) start(w http.ResponseWriter, r *http.Request) {
	if !h.allowOAuthStart(w, r) {
		return
	}
	providerName := chi.URLParam(r, "provider")
	binding, fresh, err := h.browserBinding(r)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if fresh {
		h.setBrowserBindingCookie(w, binding)
	}
	start, err := h.svc.BeginOAuth(
		r.Context(), providerName,
		safeReturnTo(r.URL.Query().Get("returnTo"), h.cfg.DefaultReturnTo),
		binding,
	)
	if err != nil {
		if errors.Is(err, usersvc.ErrUnknownProvider) {
			respondError(w, http.StatusNotFound, "unknown provider")
			return
		}
		middleware.LoggerFromContext(r.Context()).Error("oauth_start_failed", "provider", providerName, "err", err)
		respondError(w, http.StatusServiceUnavailable, "login unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, start.URL, http.StatusFound)
}

func (h *handler) allowOAuthStart(w http.ResponseWriter, r *http.Request) bool {
	if h.cfg.Limiter == nil {
		return true
	}
	ip := clientIP(r).String()
	key := "oauth-start:ip:" + hex.EncodeToString(appauth.HashOpaqueToken(ip))
	allowed, retryAfter, err := h.cfg.Limiter.AllowFixedWindow(
		r.Context(), key, h.cfg.OAuthStartLimit, h.cfg.OAuthStartWindow,
	)
	if err != nil {
		middleware.LoggerFromContext(r.Context()).Error("oauth_rate_limit_failed")
		respondError(w, http.StatusServiceUnavailable, "login unavailable")
		return false
	}
	if allowed {
		return true
	}
	seconds := int((retryAfter + time.Second - 1) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	respondError(w, http.StatusTooManyRequests, "too many login attempts")
	return false
}

func (h *handler) callback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Referrer-Policy", "no-referrer")
	providerName := chi.URLParam(r, "provider")
	q := r.URL.Query()
	if q.Get("error") != "" {
		h.redirectFailure(w, r, "cancelled")
		return
	}
	code, state := q.Get("code"), q.Get("state")
	bindingCookie, bindingErr := r.Cookie(browserBindingCookieName(h.cfg.CookieSecure))
	if code == "" || len(code) > maxOAuthCodeLength ||
		state == "" || len(state) > maxOAuthStateLength ||
		bindingErr != nil || bindingCookie.Value == "" || len(bindingCookie.Value) > maxBindingTokenLength {
		h.redirectFailure(w, r, "invalid_callback")
		return
	}

	session, returnTo, err := h.svc.CompleteOAuth(
		r.Context(), providerName, code, state, bindingCookie.Value,
		r.UserAgent(), clientIP(r),
	)
	if err != nil {
		switch {
		case errors.Is(err, usersvc.ErrUnknownProvider):
			h.redirectFailure(w, r, "unknown_provider")
		case errors.Is(err, usersvc.ErrInvalidOAuthAttempt):
			h.redirectFailure(w, r, "expired")
		default:
			middleware.LoggerFromContext(r.Context()).Error("oauth_callback_failed", "provider", providerName, "stage", "exchange_or_persist")
			h.redirectFailure(w, r, "oauth_failed")
		}
		return
	}
	h.setSessionCookie(w, session.Token, session.ExpiresAt)
	http.Redirect(w, r, safeReturnTo(returnTo, h.cfg.DefaultReturnTo), http.StatusSeeOther)
}

func (h *handler) logout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	cookie, err := r.Cookie(SessionCookieName(h.cfg.CookieSecure))
	if err != nil || cookie.Value == "" {
		h.clearSessionCookie(w)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Header.Get(CSRFHeader) != "1" {
		respondError(w, http.StatusForbidden, "csrf check failed")
		return
	}
	if err := h.svc.LogoutBrowserSession(r.Context(), cookie.Value); err != nil {
		middleware.LoggerFromContext(r.Context()).Error("logout_failed", "err", err)
		respondError(w, http.StatusServiceUnavailable, "logout unavailable")
		return
	}
	h.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) browserBinding(r *http.Request) (string, bool, error) {
	if cookie, err := r.Cookie(browserBindingCookieName(h.cfg.CookieSecure)); err == nil && cookie.Value != "" && len(cookie.Value) <= maxBindingTokenLength {
		return cookie.Value, false, nil
	}
	value, err := appauth.NewOpaqueToken()
	return value, true, err
}

func (h *handler) setBrowserBindingCookie(w http.ResponseWriter, value string) {
	http.SetCookie(w, &http.Cookie{
		Name: browserBindingCookieName(h.cfg.CookieSecure), Value: value,
		Path: "/oauth", MaxAge: 600, HttpOnly: true, Secure: h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *handler) setSessionCookie(w http.ResponseWriter, value string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name: SessionCookieName(h.cfg.CookieSecure), Value: value,
		Path: "/", Expires: expires, HttpOnly: true, Secure: h.cfg.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

func (h *handler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: SessionCookieName(h.cfg.CookieSecure), Value: "", Path: "/",
		MaxAge: -1, HttpOnly: true, Secure: h.cfg.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

func (h *handler) redirectFailure(w http.ResponseWriter, r *http.Request, code string) {
	target, err := url.Parse(h.cfg.FailureRedirect)
	if err != nil || target.IsAbs() || target.Host != "" {
		target = &url.URL{Path: "/login"}
	}
	query := target.Query()
	query.Set("error", code)
	target.RawQuery = query.Encode()
	http.Redirect(w, r, target.String(), http.StatusSeeOther)
}

func safeReturnTo(raw, fallback string) string {
	if fallback == "" {
		fallback = "/me"
	}
	if raw == "" || len(raw) > maxReturnToLength || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") || strings.Contains(raw, "\\") {
		return fallback
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return fallback
	}
	return parsed.RequestURI()
}

func clientIP(r *http.Request) netip.Addr {
	if raw := middleware.ClientIPFromContext(r.Context()); raw != "unknown" {
		if addr, err := netip.ParseAddr(raw); err == nil {
			return addr
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	addr, _ := netip.ParseAddr(host)
	return addr
}

func respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + message + `"}`))
}
