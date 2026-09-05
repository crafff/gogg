package riotapi

import (
	"context"
	"fmt"
	"net/url"
)

// SummonerDTO is the platform-scoped profile returned by Summoner-V4.
// Riot ID identity lives in Account-V1; this response supplies the profile
// icon and level used by the summoner page header.
type SummonerDTO struct {
	PUUID         string `json:"puuid"`
	ProfileIconID int    `json:"profileIconId"`
	SummonerLevel int64  `json:"summonerLevel"`
}

func (c *Client) GetSummonerByPUUID(ctx context.Context, puuid string) (*SummonerDTO, error) {
	u := fmt.Sprintf("%s/lol/summoner/v4/summoners/by-puuid/%s", c.platformURL, url.PathEscape(puuid))
	var dto SummonerDTO
	return &dto, c.doRequest(ctx, u, &dto)
}
