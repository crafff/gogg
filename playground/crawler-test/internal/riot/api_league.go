package riot

import (
	"context"
	"fmt"
	"net/url"
)

type LeagueListDTO struct {
	Tier     string `json:"tier"`
	LeagueID string `json:"leagueId"`
	Queue    string `json:"queue"`
	Name     string `json:"name"`
	Entries  []struct {
		Puuid        string `json:"puuid"`
		LeaguePoints int    `json:"leaguePoints"`
		Rank         string `json:"rank"`
		Wins         int    `json:"wins"`
		Losses       int    `json:"losses"`
		Veteran      bool   `json:"veteran"`
		Inactive     bool   `json:"inactive"`
		FreshBlood   bool   `json:"freshBlood"`
		HotStreak    bool   `json:"hotStreak"`
	} `json:"entries"`
}

// GetChallengerLeaguesByQueue 具体的 API 封装
func (c *Client) GetChallengerLeaguesByQueue(ctx context.Context, queue string) (*LeagueListDTO, error) {
	// League V4 通常使用大区路由
	url := fmt.Sprintf("https://na1.api.riotgames.com/lol/league/v4/challengerleagues/by-queue/%s", url.PathEscape(queue))

	var leagueList LeagueListDTO

	// 调用底座的 doRequest，所有的脏活累活都在里面处理完了
	err := c.doRequest(ctx, "GET", url, &leagueList)
	if err != nil {
		return nil, err
	}

	return &leagueList, nil
}
