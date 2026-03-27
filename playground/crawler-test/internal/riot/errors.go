package riot

import (
	"fmt"
	"time"
)

// RateLimitError 专门用于传递 429 超速信息
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited, retry after %v", e.RetryAfter)
}

// 用于传递400以上的Riot API错误
type RiotError struct {
	Status struct {
		Message    string `json:"message"`
		StatusCode int    `json:"status_code"`
	} `json:"status"`
}

func (e *RiotError) Error() string {
	return fmt.Sprintf("Riot API error: %s (status code: %d)", e.Status.Message, e.Status.StatusCode)
}
