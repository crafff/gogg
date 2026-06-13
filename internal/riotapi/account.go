package riotapi

import (
	"context"
	"fmt"
	"net/url"
)

type AccountDTO struct {
	Puuid    string `json:"puuid"`
	GameName string `json:"gameName"`
	TagLine  string `json:"tagLine"`
}

func (c *Client) GetAccountByPUUID(ctx context.Context, puuid string) (*AccountDTO, error) {
	u := fmt.Sprintf("%s/riot/account/v1/accounts/by-puuid/%s", c.regionalURL, url.PathEscape(puuid))
	var dto AccountDTO
	return &dto, c.doRequest(ctx, u, &dto)
}

func (c *Client) GetAccountByRiotID(ctx context.Context, gameName, tagLine string) (*AccountDTO, error) {
	u := fmt.Sprintf("%s/riot/account/v1/accounts/by-riot-id/%s/%s",
		c.regionalURL, url.PathEscape(gameName), url.PathEscape(tagLine))
	var dto AccountDTO
	return &dto, c.doRequest(ctx, u, &dto)
}
