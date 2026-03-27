package riot

import (
	"context"
	"fmt"
	"net/url"
)

// AccountDTO 与业务强绑定的数据结构
type AccountDTO struct {
	Puuid    string `json:"puuid"`
	GameName string `json:"gameName"`
	TagLine  string `json:"tagLine"`
}

// GetAccountByRiotID 具体的 API 封装
func (c *Client) GetAccountByRiotID(ctx context.Context, gameName, tagLine string) (*AccountDTO, error) {
	// Account V1 通常使用大区路由
	url := fmt.Sprintf("https://americas.api.riotgames.com/riot/account/v1/accounts/by-riot-id/%s/%s", url.PathEscape(gameName), url.PathEscape(tagLine))

	var account AccountDTO

	// 调用底座的 doRequest，所有的脏活累活都在里面处理完了
	err := c.doRequest(ctx, "GET", url, &account)
	if err != nil {
		return nil, err
	}

	return &account, nil
}
