package riotapi

import (
	"fmt"
	"time"
)

type RateLimitError struct {
	RetryAfter time.Duration
}

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
