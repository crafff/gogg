package riotapi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
)

// Client wraps the Riot API HTTP client with rate limiting.
// platformURL: platform-level routing, e.g. https://kr.api.riotgames.com
// regionalURL: regional routing, e.g.  https://asia.api.riotgames.com
type Client struct {
	apiKey      string
	platformURL string
	regionalURL string
	limiter     *RateLimiter
	http        *http.Client
}

func NewClient(apiKey, platformURL, regionalURL string) *Client {
	return &Client{
		apiKey:      apiKey,
		platformURL: platformURL,
		regionalURL: regionalURL,
		limiter:     NewRateLimiter(),
		http:        &http.Client{Timeout: 10 * time.Second},
	}
}

const maxRetries = 5

func (c *Client) doRequest(ctx context.Context, url string, result any) error {
	for attempt := range maxRetries {
		if err := c.limiter.Wait(ctx); err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("X-Riot-Token", c.apiKey)

		resp, err := c.http.Do(req)
		if err != nil {
			return err
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			retrySec, _ := strconv.Atoi(resp.Header.Get("Retry-After"))
			resp.Body.Close()
			if retrySec <= 0 {
				retrySec = 10
			}
			wait := time.Duration(retrySec) * time.Second
			slog.WarnContext(ctx, "rate limited by Riot API",
				"retry_after_s", retrySec, "attempt", attempt+1)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
			continue
		}

		if resp.StatusCode >= 400 {
			var riotErr RiotError
			_ = sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&riotErr)
			resp.Body.Close()
			return &riotErr
		}

		if result != nil {
			err = sonic.ConfigDefault.NewDecoder(resp.Body).Decode(result)
		}
		resp.Body.Close()
		return err
	}
	return fmt.Errorf("riot API request failed after %d retries: %s", maxRetries, url)
}
