package riotapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
)

type RetryPolicy struct {
	// MaxAttempts is the total number of requests. Zero means retry forever.
	MaxAttempts     int
	InitialInterval time.Duration
	MaximumInterval time.Duration
	Jitter          float64
}

var defaultRetryPolicy = RetryPolicy{
	MaxAttempts:     5,
	InitialInterval: time.Second,
	MaximumInterval: 30 * time.Second,
	Jitter:          0.2,
}

var outageRetryPolicy = RetryPolicy{
	MaxAttempts:     0,
	InitialInterval: time.Second,
	MaximumInterval: 2 * time.Minute,
	Jitter:          0.2,
}

// Client wraps the Riot API HTTP client with rate limiting.
type Client struct {
	apiKey      string
	platformURL string
	regionalURL string
	limiter     *RateLimiter
	http        *http.Client
	retry       RetryPolicy
}

func NewClient(apiKey, platformURL, regionalURL string) *Client {
	return &Client{
		apiKey:      apiKey,
		platformURL: platformURL,
		regionalURL: regionalURL,
		limiter:     NewRateLimiter(),
		http:        &http.Client{Timeout: 15 * time.Second},
		retry:       defaultRetryPolicy,
	}
}

// EnableOutageWait makes transient network and Riot server errors wait until
// the caller cancels the context. Configure clients before they are shared.
func (c *Client) EnableOutageWait() { c.retry = outageRetryPolicy }

func (c *Client) ConfigureOutageWait(initialInterval, maximumInterval time.Duration, jitterFraction float64) error {
	if initialInterval <= 0 || maximumInterval < initialInterval || jitterFraction < 0 || jitterFraction > 1 {
		return fmt.Errorf("invalid outage retry policy: initial=%s maximum=%s jitter=%v",
			initialInterval, maximumInterval, jitterFraction)
	}
	c.retry = RetryPolicy{
		MaxAttempts:     0,
		InitialInterval: initialInterval,
		MaximumInterval: maximumInterval,
		Jitter:          jitterFraction,
	}
	return nil
}

func (c *Client) doRequest(ctx context.Context, requestURL string, result any) error {
	interval := c.retry.InitialInterval
	for attempt := 1; ; attempt++ {
		err := c.doOnce(ctx, requestURL, result)
		if err == nil {
			if attempt > 1 {
				slog.InfoContext(ctx, "riot_connectivity_restored", "attempt", attempt)
			}
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var apiErr *APIError
		if !errors.As(err, &apiErr) || !apiErr.Retryable ||
			(apiErr.Kind == ErrorDecode && attempt >= 3) ||
			(c.retry.MaxAttempts > 0 && attempt >= c.retry.MaxAttempts) {
			return err
		}

		wait := interval
		if apiErr.RetryAfter > wait {
			wait = apiErr.RetryAfter
		}
		wait = jitter(wait, c.retry.Jitter)
		slog.WarnContext(ctx, "riot_retry_wait", "kind", apiErr.Kind, "status", apiErr.StatusCode,
			"attempt", attempt, "wait", wait, "err", err)
		if err := waitContext(ctx, wait); err != nil {
			return err
		}
		interval *= 2
		if interval > c.retry.MaximumInterval {
			interval = c.retry.MaximumInterval
		}
	}
}

func (c *Client) doOnce(ctx context.Context, requestURL string, result any) error {
	if err := c.limiter.Wait(ctx); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return &APIError{Kind: ErrorClient, Global: true, Cause: err}
	}
	req.Header.Set("X-Riot-Token", c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return connectivityError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return responseError(resp)
	}
	if result == nil {
		_, err = io.Copy(io.Discard, resp.Body)
		return err
	}
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(result); err != nil {
		return &APIError{Kind: ErrorDecode, Retryable: true, Global: true, Cause: err}
	}
	return nil
}

func responseError(resp *http.Response) error {
	var payload RiotError
	_ = sonic.ConfigDefault.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&payload)
	message := strings.TrimSpace(payload.Status.Message)
	if message == "" {
		message = http.StatusText(resp.StatusCode)
	}
	err := &APIError{StatusCode: resp.StatusCode, Message: message}
	if resp.StatusCode >= http.StatusInternalServerError {
		err.Kind, err.Retryable, err.Global = ErrorServer, true, true
		return err
	}
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		err.Kind, err.Global = ErrorUnauthorized, true
	case http.StatusNotFound:
		err.Kind = ErrorNotFound
	case http.StatusRequestTimeout, http.StatusTooEarly:
		err.Kind, err.Retryable, err.Global = ErrorServer, true, true
	case http.StatusTooManyRequests:
		err.Kind, err.Retryable, err.Global = ErrorRateLimit, true, true
		err.RetryAfter = parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
	default:
		err.Kind, err.Global = ErrorClient, true
	}
	return err
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if retryAt, err := http.ParseTime(value); err == nil && retryAt.After(now) {
		return retryAt.Sub(now)
	}
	return 0
}

func waitContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func jitter(d time.Duration, fraction float64) time.Duration {
	if fraction <= 0 || d <= 0 {
		return d
	}
	factor := 1 + ((rand.Float64()*2 - 1) * fraction)
	return time.Duration(float64(d) * factor)
}
