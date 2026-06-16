// Package domainerr is the small "this error is safe to show the
// caller" wrapper the GraphQL error presenter looks for. Service code
// returns these for problems the client can react to (bad filter,
// no data for the requested patch, rate limited). The presenter
// turns them into a GraphQL error with a stable `code` extension and
// a human-readable message; everything else collapses to
// "internal server error".
//
// V1 doesn't use this from the service layer yet — chunk 5 ships
// only the wrapper + presenter so future service-layer code can
// surface client-safe errors without touching the transport. Phase E
// (champion detail, summoner search, user system) will be the first
// real users.
package domainerr

import "fmt"

// Error is a safe-to-publish error. Public is the message the GraphQL
// caller sees; Code is the machine-readable extension. Wraps the
// underlying error so errors.Is / errors.As still work for tests.
type Error struct {
	Code   string
	Public string
	cause  error
}

// New builds a domain error with a generic underlying cause.
func New(code, public string) *Error {
	return &Error{Code: code, Public: public}
}

// Wrap attaches an underlying cause. Useful when the service layer
// translates a sentinel (e.g. errors.Is(err, pgx.ErrNoRows)) into a
// public message but wants the original kept for the logs.
func Wrap(code, public string, cause error) *Error {
	return &Error{Code: code, Public: public, cause: cause}
}

// Error implements the error interface. Renders the internal form
// ("code: public") — the public-facing message is rebuilt by the
// presenter from the Public field.
func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Public, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Public)
}

// Unwrap exposes the cause to errors.Is / errors.As.
func (e *Error) Unwrap() error { return e.cause }
