package riotapi

import (
    "context"
    "net/http"
    "golang.org/x/time/rate"
)

type Client struct {
    httpClient *http.Client
    apiKey     string
    // 限流器：比如开发版 Key 每秒 20 个请求
    limiter    *rate.Limiter 
}

func NewClient(apiKey string) *Client {
    return &Client{
        httpClient: &http.Client{},
        apiKey:     apiKey,
        limiter:    rate.NewLimiter(rate.Limit(20), 1), // 每秒 20 个令牌
    }
}

func (c *Client) DoRequest(req *http.Request) (*http.Response, error) {
    // 关键：在发起请求前，先等限流器“放行”
    err := c.limiter.Wait(context.Background())
    if err != nil {
        return nil, err
    }
    
    req.Header.Set("X-Riot-Token", c.apiKey)
    return c.httpClient.Do(req)
}