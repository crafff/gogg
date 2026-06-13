package riotapi

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// GetMatchIDsByPUUID returns match IDs for a player, filtered by queue and time window.
// startTime / endTime are Unix epoch seconds (0 = omit).
// start is the pagination offset (0-indexed), count max 100.
func (c *Client) GetMatchIDsByPUUID(ctx context.Context, puuid string, queue int, startTime, endTime int64, start, count int) ([]string, error) {
	base := fmt.Sprintf("%s/lol/match/v5/matches/by-puuid/%s/ids", c.regionalURL, url.PathEscape(puuid))

	var params []string
	if queue > 0 {
		params = append(params, fmt.Sprintf("queue=%d", queue))
	}
	if startTime > 0 {
		params = append(params, fmt.Sprintf("startTime=%d", startTime))
	}
	if endTime > 0 {
		params = append(params, fmt.Sprintf("endTime=%d", endTime))
	}
	if start > 0 {
		params = append(params, fmt.Sprintf("start=%d", start))
	}
	if count <= 0 || count > 100 {
		count = 100
	}
	params = append(params, fmt.Sprintf("count=%d", count))

	u := base
	if len(params) > 0 {
		u += "?" + strings.Join(params, "&")
	}

	var ids []string
	return ids, c.doRequest(ctx, u, &ids)
}

// GetMatchDetail returns the full match detail for a match ID.
func (c *Client) GetMatchDetail(ctx context.Context, matchID string) (*MatchDetailDTO, error) {
	u := fmt.Sprintf("%s/lol/match/v5/matches/%s", c.regionalURL, url.PathEscape(matchID))
	var dto MatchDetailDTO
	return &dto, c.doRequest(ctx, u, &dto)
}
