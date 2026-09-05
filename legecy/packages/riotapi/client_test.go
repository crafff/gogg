package riotapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestDoRequestRetriesServerErrors(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) < 3 {
			http.Error(w, `{"status":{"message":"unavailable","status_code":503}}`, http.StatusServiceUnavailable)
			return
		}
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()

	client := testClient(server.URL)
	var result struct {
		Ok bool `json:"ok"`
	}
	if err := client.doRequest(context.Background(), server.URL, &result); err != nil {
		t.Fatalf("doRequest: %v", err)
	}
	if !result.Ok || requests.Load() != 3 {
		t.Fatalf("result=%+v requests=%d", result, requests.Load())
	}
}

func TestDoRequestDoesNotRetryUnauthorized(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"status":{"message":"Forbidden","status_code":401}}`)
	}))
	defer server.Close()

	client := testClient(server.URL)
	err := client.doRequest(context.Background(), server.URL, nil)
	if !IsGlobalPermanent(err) {
		t.Fatalf("expected global permanent error, got %v", err)
	}
	if requests.Load() != 1 {
		t.Fatalf("expected one request, got %d", requests.Load())
	}
}

func TestOutageWaitStopsOnContextCancellation(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := testClient(server.URL)
	client.retry.MaxAttempts = 0
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	err := client.doRequest(ctx, server.URL, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	if requests.Load() < 2 {
		t.Fatalf("expected retries before cancellation, got %d request(s)", requests.Load())
	}
}

func TestOutageWaitRecoversAfterNetworkReturns(t *testing.T) {
	client := testClient("https://riot.test")
	client.retry.MaxAttempts = 0
	transport := &recoveringTransport{failuresRemaining: 3}
	client.http.Transport = transport

	var result struct {
		Ok bool `json:"ok"`
	}
	if err := client.doRequest(context.Background(), "https://riot.test/resource", &result); err != nil {
		t.Fatalf("doRequest: %v", err)
	}
	if !result.Ok {
		t.Fatal("request did not continue after connectivity returned")
	}
	if got := transport.requests.Load(); got != 4 {
		t.Fatalf("expected four attempts, got %d", got)
	}
}

func TestOutageWaitLimitsDecodeRetries(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		fmt.Fprint(w, `{not-json`)
	}))
	defer server.Close()

	client := testClient(server.URL)
	client.retry.MaxAttempts = 0
	var result map[string]any
	err := client.doRequest(context.Background(), server.URL, &result)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Kind != ErrorDecode {
		t.Fatalf("expected decode APIError, got %v", err)
	}
	if requests.Load() != 3 {
		t.Fatalf("expected three decode attempts, got %d", requests.Load())
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, time.July, 12, 12, 0, 0, 0, time.UTC)
	if got := parseRetryAfter("12", now); got != 12*time.Second {
		t.Fatalf("seconds Retry-After = %v", got)
	}
	retryAt := now.Add(30 * time.Second).Format(http.TimeFormat)
	if got := parseRetryAfter(retryAt, now); got != 30*time.Second {
		t.Fatalf("date Retry-After = %v", got)
	}
}

func TestConfigureOutageWaitRejectsInvalidPolicy(t *testing.T) {
	client := NewClient("test-key", "https://example.test", "https://example.test")
	if err := client.ConfigureOutageWait(2*time.Second, time.Second, 0.2); err == nil {
		t.Fatal("expected invalid interval error")
	}
	if err := client.ConfigureOutageWait(time.Second, time.Minute, 1.1); err == nil {
		t.Fatal("expected invalid jitter error")
	}
	if err := client.ConfigureOutageWait(time.Second, time.Minute, 0.2); err != nil {
		t.Fatalf("valid policy: %v", err)
	}
	if client.retry.MaxAttempts != 0 || client.retry.MaximumInterval != time.Minute {
		t.Fatalf("unexpected configured policy: %+v", client.retry)
	}
}

func testClient(baseURL string) *Client {
	client := NewClient("test-key", baseURL, baseURL)
	client.retry = RetryPolicy{
		MaxAttempts:     5,
		InitialInterval: time.Millisecond,
		MaximumInterval: 2 * time.Millisecond,
	}
	client.limiter = &RateLimiter{
		perSecond: rate.NewLimiter(rate.Inf, 1),
		per2Min:   rate.NewLimiter(rate.Inf, 1),
	}
	return client
}

type recoveringTransport struct {
	failuresRemaining int32
	requests          atomic.Int32
}

func (t *recoveringTransport) RoundTrip(*http.Request) (*http.Response, error) {
	attempt := t.requests.Add(1)
	if attempt <= t.failuresRemaining {
		return nil, &net.DNSError{Err: "network is unreachable", Name: "riot.test", IsTemporary: true}
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
	}, nil
}
