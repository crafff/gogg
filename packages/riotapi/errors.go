package riotapi

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"
)

type ErrorKind string

const (
	ErrorConnectivity ErrorKind = "connectivity"
	ErrorServer       ErrorKind = "server"
	ErrorRateLimit    ErrorKind = "rate_limit"
	ErrorUnauthorized ErrorKind = "unauthorized"
	ErrorNotFound     ErrorKind = "not_found"
	ErrorClient       ErrorKind = "client"
	ErrorDecode       ErrorKind = "decode"
)

// APIError describes how callers should handle a failed Riot request.
type APIError struct {
	Kind       ErrorKind
	StatusCode int
	RetryAfter time.Duration
	Retryable  bool
	Global     bool
	Message    string
	Cause      error
}

func (e *APIError) Error() string {
	if e.StatusCode != 0 {
		return fmt.Sprintf("riot API error %d: %s", e.StatusCode, e.Message)
	}
	if e.Cause != nil {
		return fmt.Sprintf("riot API %s error: %v", e.Kind, e.Cause)
	}
	return fmt.Sprintf("riot API %s error: %s", e.Kind, e.Message)
}

func (e *APIError) Unwrap() error { return e.Cause }

func IsRetryable(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Retryable
}

func IsGlobalPermanent(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Global && !apiErr.Retryable
}

func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Kind == ErrorNotFound
}

// IsUnauthorized reports whether Riot rejected the configured API key.
func IsUnauthorized(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Kind == ErrorUnauthorized
}

func connectivityError(err error) *APIError {
	if err == nil {
		return nil
	}
	var urlErr *url.Error
	var netErr net.Error
	if errors.As(err, &urlErr) && errors.As(urlErr.Err, &netErr) {
		return &APIError{Kind: ErrorConnectivity, Retryable: true, Global: true, Cause: err}
	}
	if errors.As(err, &netErr) {
		return &APIError{Kind: ErrorConnectivity, Retryable: true, Global: true, Cause: err}
	}
	return &APIError{Kind: ErrorClient, Global: true, Cause: err}
}

// Kept as aliases for callers which used the old exported error types.
type RateLimitError struct{ RetryAfter time.Duration }

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited, retry after %v", e.RetryAfter)
}

type RiotError struct {
	Status struct {
		Message    string `json:"message"`
		StatusCode int    `json:"status_code"`
	} `json:"status"`
}

func (e *RiotError) Error() string {
	return fmt.Sprintf("riot API error %d: %s", e.Status.StatusCode, e.Status.Message)
}
