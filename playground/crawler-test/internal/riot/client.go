package riot

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
)

type Client struct {
	apiKey     string
	httpClient *http.Client
}

func NewClient(apiKey string) *Client {

	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 6 * time.Second}, // Timeout包含链接和读取Body的时间，防止长时间挂起（慢速连接攻击或死锁）
	}
}

// doRequest 是所有具体业务函数的核心引擎
// method: GET/POST, url: 完整路径, result: 传入指针用于 JSON 反序列化
func (c *Client) doRequest(ctx context.Context, method, url string, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return err
	}

	// 统一鉴权注入
	req.Header.Set("X-Riot-Token", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 统一的状态码拦截与错误翻译
	if resp.StatusCode == http.StatusTooManyRequests { // 429
		retrySec, _ := strconv.Atoi(resp.Header.Get("Retry-After"))
		if retrySec == 0 {
			retrySec = 1 // 兜底保护
		}
		return &RateLimitError{RetryAfter: time.Duration(retrySec) * time.Second}
	}

	if resp.StatusCode >= 400 {
		var riotErr RiotError
		if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&riotErr); err != nil {
			return err
		}
		return &riotErr
	}

	// 统一的 JSON 反序列化
	if result != nil {
		if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(result); err != nil {
			return err
		}
	}

	return nil
}
