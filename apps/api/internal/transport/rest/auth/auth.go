// Package auth holds the REST handlers for the OAuth + token endpoints:
//
//	GET  /oauth/start/{provider}    — kick off the consent redirect
//	GET  /oauth/callback/{provider} — exchange code, issue session
//	POST /auth/refresh              — rotate the refresh token
//	POST /auth/logout               — revoke the current refresh token
//
// The handlers are thin: they just parse the cookie / query / body
// and forward to apps/api/internal/service/user. The session bundle
// returned by the service maps to:
//
//	access token  → JSON body { accessToken, expiresAt, user }
//	refresh token → HttpOnly cookie ("gogg_refresh")
//
// Storing the refresh in a cookie keeps it out of JS reach (xss
// mitigation); the access lives in JS memory and dies with the tab.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/crafff/gogg/apps/api/internal/service/user"
	"github.com/crafff/gogg/apps/api/internal/transport/middleware"
)

// Service is the consumer-side interface the handlers depend on.
// Implemented by *user.Service in practice; the abstraction is here
// just so tests don't need real DB + JWT machinery.
type Service interface {
	AuthCodeURL(provider, state string) (string, error)
	LoginFromOAuth(ctx context.Context, provider, code, userAgent string, ip netip.Addr) (user.Session, error)
	RefreshSession(ctx context.Context, refreshToken, userAgent string, ip netip.Addr) (user.Session, error)
	Logout(ctx context.Context, refreshToken string) error
}

// Config controls cookie + redirect behaviour. CookieDomain blank →
// host-only cookie (recommended for V1; covers api.gogg.gg without
// also leaking to other subdomains). SuccessRedirect is the SPA route
// the user lands on after a successful login; the SPA reads the
// access token out of the JSON body of /oauth/callback's redirected
// fragment in V1, switching to PKCE in Phase F.
type Config struct {
	CookieDomain    string
	CookieSecure    bool // set false for local http; true for prod https
	SuccessRedirect string
	FailureRedirect string
}

// Routes returns the chi sub-router. Mount with r.Mount("/", auth.Routes(...))
// since the paths span /oauth/* and /auth/*.
func Routes(svc Service, cfg Config) chi.Router {
	r := chi.NewRouter()
	h := &handler{svc: svc, cfg: cfg}
	r.Get("/oauth/start/{provider}", h.start)
	r.Get("/oauth/callback/{provider}", h.callback)
	r.Post("/auth/refresh", h.refresh)
	r.Post("/auth/logout", h.logout)
	return r
}

type handler struct {
	svc Service
	cfg Config
}

const (
	refreshCookieName = "gogg_refresh"
	stateCookieName   = "gogg_oauth_state"
)

// start redirects the browser to the provider's consent screen. A
// random state value is stored in a short-lived cookie and verified
// on callback to defeat CSRF.
func (h *handler) start(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")
	state, err := newState()
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	url, err := h.svc.AuthCodeURL(providerName, state)
	if err != nil {
		respondError(w, r, http.StatusNotFound, "unknown provider")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    state,
		Path:     "/oauth",
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})
	http.Redirect(w, r, url, http.StatusFound)
}

// callback completes the OAuth flow.
func (h *handler) callback(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")

	q := r.URL.Query()
	code := q.Get("code")
	state := q.Get("state")
	if code == "" || state == "" {
		respondError(w, r, http.StatusBadRequest, "missing code or state")
		return
	}
	stateCookie, err := r.Cookie(stateCookieName)
	if err != nil || stateCookie.Value == "" || stateCookie.Value != state {
		respondError(w, r, http.StatusBadRequest, "state mismatch")
		return
	}
	// Single-use: clear the state cookie before doing anything else
	// so a leaked callback URL is not replayable.
	h.clearCookie(w, stateCookieName, "/oauth")

	sess, err := h.svc.LoginFromOAuth(r.Context(), providerName, code, r.UserAgent(), clientIP(r))
	if err != nil {
		if errors.Is(err, user.ErrUnknownProvider) {
			respondError(w, r, http.StatusNotFound, "unknown provider")
			return
		}
		middleware.LoggerFromContext(r.Context()).Error("oauth_login_failed", "provider", providerName, "err", err)
		respondError(w, r, http.StatusBadGateway, "oauth login failed")
		return
	}

	h.setRefreshCookie(w, sess.RefreshToken, sess.RefreshExpires)

	// V1: respond with JSON so the SPA can render. Phase F adds a
	// proper PKCE + redirect flow with a one-shot landing page.
	respondJSON(w, http.StatusOK, map[string]any{
		"accessToken":     sess.AccessToken,
		"accessExpiresAt": sess.AccessExpiresAt,
		"userId":          sess.UserID.String(),
	})
}

// refresh rotates the refresh token. The new access goes back in the
// JSON body; the new refresh goes back in the same HttpOnly cookie.
func (h *handler) refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(refreshCookieName)
	if err != nil || cookie.Value == "" {
		respondError(w, r, http.StatusUnauthorized, "no refresh token")
		return
	}
	sess, err := h.svc.RefreshSession(r.Context(), cookie.Value, r.UserAgent(), clientIP(r))
	if err != nil {
		if errors.Is(err, user.ErrInvalidRefresh) {
			respondError(w, r, http.StatusUnauthorized, "refresh token invalid or expired")
			return
		}
		middleware.LoggerFromContext(r.Context()).Error("refresh_failed", "err", err)
		respondError(w, r, http.StatusInternalServerError, "refresh failed")
		return
	}
	h.setRefreshCookie(w, sess.RefreshToken, sess.RefreshExpires)
	respondJSON(w, http.StatusOK, map[string]any{
		"accessToken":     sess.AccessToken,
		"accessExpiresAt": sess.AccessExpiresAt,
		"userId":          sess.UserID.String(),
	})
}

// logout revokes the current refresh token and clears the cookie.
// Idempotent — calling without a cookie still returns 204.
func (h *handler) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(refreshCookieName); err == nil && cookie.Value != "" {
		if err := h.svc.Logout(r.Context(), cookie.Value); err != nil {
			middleware.LoggerFromContext(r.Context()).Error("logout_failed", "err", err)
		}
	}
	h.clearCookie(w, refreshCookieName, "/")
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) setRefreshCookie(w http.ResponseWriter, value string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    value,
		Path:     "/",
		Domain:   h.cfg.CookieDomain,
		Expires:  expires,
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *handler) clearCookie(w http.ResponseWriter, name, path string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     path,
		Domain:   h.cfg.CookieDomain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// newState returns a 32-byte url-safe random — fits the same bucket as
// auth.NewOpaqueToken but it's deliberately inlined here so the auth
// package doesn't get pulled into transport for one helper.
func newState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// clientIP extracts the request origin. Honours X-Forwarded-For when
// the request came through a trusted proxy (i.e. always in prod since
// the api sits behind an ingress); falls back to RemoteAddr otherwise.
// Phase F will add a "trust this many proxy hops" config knob — V1
// trusts the immediate X-Forwarded-For first entry, which is what the
// nginx default produces.
func clientIP(r *http.Request) netip.Addr {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		first := strings.TrimSpace(strings.SplitN(v, ",", 2)[0])
		if a, err := netip.ParseAddr(first); err == nil {
			return a
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	a, _ := netip.ParseAddr(host)
	return a
}

func respondError(w http.ResponseWriter, r *http.Request, code int, msg string) {
	middleware.LoggerFromContext(r.Context()).Warn("auth_rest_error", "code", code, "msg", msg)
	respondJSON(w, code, map[string]string{"error": msg})
}

func respondJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
