package riotapi

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

func (c *Client) GetTFTChallengerLeague(ctx context.Context, queue string) (*TFTLeagueListDTO, error) {
	return c.getTFTTopLeague(ctx, "challenger", queue)
}

func (c *Client) GetTFTGrandmasterLeague(ctx context.Context, queue string) (*TFTLeagueListDTO, error) {
	return c.getTFTTopLeague(ctx, "grandmaster", queue)
}

func (c *Client) GetTFTMasterLeague(ctx context.Context, queue string) (*TFTLeagueListDTO, error) {
	return c.getTFTTopLeague(ctx, "master", queue)
}

func (c *Client) getTFTTopLeague(ctx context.Context, tier, queue string) (*TFTLeagueListDTO, error) {
	u := fmt.Sprintf("%s/tft/league/v1/%s?queue=%s", c.platformURL, url.PathEscape(tier), url.QueryEscape(queue))
	var dto TFTLeagueListDTO
	meta := ResponseMeta{Kind: "tft-league-" + tier, ResourceKey: strings.ToUpper(queue), Operation: "tft-league-get-" + tier}
	return &dto, c.doRequestRawFirst(ctx, u, &dto, meta)
}

// GetTFTLeagueEntries returns one 1-indexed Diamond ladder page.
func (c *Client) GetTFTLeagueEntries(ctx context.Context, queue, tier, division string, page int) ([]TFTLeagueEntryDTO, error) {
	if page < 1 {
		page = 1
	}
	u := fmt.Sprintf("%s/tft/league/v1/entries/%s/%s?queue=%s&page=%d",
		c.platformURL, url.PathEscape(strings.ToUpper(tier)), url.PathEscape(strings.ToUpper(division)),
		url.QueryEscape(queue), page)
	var entries []TFTLeagueEntryDTO
	resource := fmt.Sprintf("%s/%s/%d", strings.ToUpper(tier), strings.ToUpper(division), page)
	return entries, c.doRequestRawFirst(ctx, u, &entries, ResponseMeta{Kind: "tft-league-page", ResourceKey: resource, Operation: "tft-league-get-entries"})
}

func (c *Client) GetTFTSummonerByPUUID(ctx context.Context, puuid string) (*TFTSummonerDTO, error) {
	u := fmt.Sprintf("%s/tft/summoner/v1/summoners/by-puuid/%s", c.platformURL, url.PathEscape(puuid))
	var dto TFTSummonerDTO
	return &dto, c.doRequestRawFirst(ctx, u, &dto, ResponseMeta{Kind: "tft-summoner", ResourceKey: puuid, Operation: "tft-summoner-get-by-puuid"})
}

// GetTFTMatchIDsByPUUID returns a time-bounded page. Riot does not expose a
// queue filter for this endpoint; callers must filter queue_id after detail.
func (c *Client) GetTFTMatchIDsByPUUID(ctx context.Context, puuid string, startTime, endTime int64, start, count int) ([]string, error) {
	if start < 0 {
		start = 0
	}
	if count <= 0 || count > 100 {
		count = 20
	}
	params := url.Values{}
	params.Set("start", fmt.Sprint(start))
	params.Set("count", fmt.Sprint(count))
	if startTime > 0 {
		params.Set("startTime", fmt.Sprint(startTime))
	}
	if endTime > 0 {
		params.Set("endTime", fmt.Sprint(endTime))
	}
	u := fmt.Sprintf("%s/tft/match/v1/matches/by-puuid/%s/ids?%s", c.regionalURL, url.PathEscape(puuid), params.Encode())
	var ids []string
	resource := fmt.Sprintf("%s/%d/%d/%d/%d", puuid, startTime, endTime, start, count)
	return ids, c.doRequestRawFirst(ctx, u, &ids, ResponseMeta{Kind: "tft-match-ids", ResourceKey: resource, Operation: "tft-match-get-ids-by-puuid"})
}

func (c *Client) GetTFTMatchDetail(ctx context.Context, matchID string) (*TFTMatchDTO, error) {
	u := fmt.Sprintf("%s/tft/match/v1/matches/%s", c.regionalURL, url.PathEscape(matchID))
	var dto TFTMatchDTO
	meta := ResponseMeta{Kind: "tft-match-detail", MatchID: matchID, ResourceKey: matchID, Operation: "tft-match-get-detail"}
	return &dto, c.doRequestRawFirst(ctx, u, &dto, meta)
}
