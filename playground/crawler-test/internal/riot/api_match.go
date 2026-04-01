package riot

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

type Matchs []string

// GetMatchsByPUUID 具体的 API 封装
func (c *Client) GetMatchsByPuuid(ctx context.Context, puuid string, star_time int64, 
		end_time int64, match_type string, start int, count int,) (*Matchs, error) {
	// Match V5 使用大区路由
	
	requestURL := fmt.Sprintf("https://americas.api.riotgames.com/lol/match/v5/matches/by-puuid/%s/ids", url.PathEscape(puuid))

	var params []string

	if star_time > 0 {
		params = append(params, fmt.Sprintf("startTime=%d", star_time))
	}

	if end_time >= 0 {
		params = append(params, fmt.Sprintf("endTime=%d", end_time))
	}

	if match_type != "" {
		params = append(params, fmt.Sprintf("type=%s", url.QueryEscape(match_type)))
	}

	if start > 0 {
		params = append(params, fmt.Sprintf("start=%d", start))
	}

	if count > 100 {
		count = 100
	}
	if count > 0 {
		params = append(params, fmt.Sprintf("count=%d", count))
	}

	if match_type != "" {
		params = append(params, fmt.Sprintf("type=%s", url.QueryEscape(match_type)))
	}

	if len(params) > 0 {
		requestURL += "?" + strings.Join(params, "&")
	}

	// fmt.Printf("Request URL: %s\n", requestURL)

	var matchs Matchs

	// 调用底座的 doRequest，所有的脏活累活都在里面处理完了
	err := c.doRequest(ctx, "GET", requestURL, &matchs)
	if err != nil {
		return nil, err
	}

	return &matchs, nil
}