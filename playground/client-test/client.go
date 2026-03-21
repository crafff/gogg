package main // 保持和 main.go 同一个包名

import (
	"context"
	"net/http"
	"golang.org/x/time/rate"
)

// Client 结构体定义
type Client struct {
	httpClient *http.Client
	apiKey     string
	limiter    *rate.Limiter
}

// NewClient 构造函数
func NewClient(apiKey string, rps int) *Client {
	return &Client{
		httpClient: &http.Client{},
		apiKey:     apiKey,
		limiter:    rate.NewLimiter(rate.Limit(rps), 1),
	}
}

// DoRequest 发送请求逻辑
func (c *Client) DoRequest(req *http.Request) (*http.Response, error) {
	err := c.limiter.Wait(context.Background())
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Riot-Token", c.apiKey)
	return c.httpClient.Do(req)
}