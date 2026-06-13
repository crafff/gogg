package riotapi

import (
	"context"
	"fmt"
	"net/url"
)

type LeagueListDTO struct {
	LeagueID string          `json:"leagueId"`
	Tier     string          `json:"tier"`
	Queue    string          `json:"queue"`
	Name     string          `json:"name"`
	Entries  []LeagueItemDTO `json:"entries"`
}

type LeagueItemDTO struct {
	Puuid        string `json:"puuid"`
	LeagueID     string `json:"leagueId"`
	LeaguePoints int    `json:"leaguePoints"`
	Rank         string `json:"rank"`
	Wins         int    `json:"wins"`
	Losses       int    `json:"losses"`
	Veteran      bool   `json:"veteran"`
	Inactive     bool   `json:"inactive"`
	FreshBlood   bool   `json:"freshBlood"`
	HotStreak    bool   `json:"hotStreak"`
}

type LeagueEntryDTO struct {
	LeagueID     string `json:"leagueId"`
	QueueType    string `json:"queueType"`
	Tier         string `json:"tier"`
	Rank         string `json:"rank"`
	Puuid        string `json:"puuid"`
	LeaguePoints int    `json:"leaguePoints"`
	Wins         int    `json:"wins"`
	Losses       int    `json:"losses"`
	Veteran      bool   `json:"veteran"`
	Inactive     bool   `json:"inactive"`
	FreshBlood   bool   `json:"freshBlood"`
	HotStreak    bool   `json:"hotStreak"`
}

func (c *Client) GetChallengerLeagues(ctx context.Context, queue string) (*LeagueListDTO, error) {
	u := fmt.Sprintf("%s/lol/league/v4/challengerleagues/by-queue/%s", c.platformURL, url.PathEscape(queue))
	var dto LeagueListDTO
	return &dto, c.doRequest(ctx, u, &dto)
}

func (c *Client) GetGrandmasterLeagues(ctx context.Context, queue string) (*LeagueListDTO, error) {
	u := fmt.Sprintf("%s/lol/league/v4/grandmasterleagues/by-queue/%s", c.platformURL, url.PathEscape(queue))
	var dto LeagueListDTO
	return &dto, c.doRequest(ctx, u, &dto)
}

func (c *Client) GetMasterLeagues(ctx context.Context, queue string) (*LeagueListDTO, error) {
	u := fmt.Sprintf("%s/lol/league/v4/masterleagues/by-queue/%s", c.platformURL, url.PathEscape(queue))
	var dto LeagueListDTO
	return &dto, c.doRequest(ctx, u, &dto)
}

// GetLeagueEntries returns a page of entries for the given tier/division.
// page is 1-indexed.
func (c *Client) GetLeagueEntries(ctx context.Context, queue, tier, division string, page int) ([]LeagueEntryDTO, error) {
	u := fmt.Sprintf("%s/lol/league/v4/entries/%s/%s/%s?page=%d",
		c.platformURL, url.PathEscape(queue), url.PathEscape(tier), url.PathEscape(division), page)
	var entries []LeagueEntryDTO
	return entries, c.doRequest(ctx, u, &entries)
}

// GetEntriesByPUUID returns the ranked entries for a specific player.
func (c *Client) GetEntriesByPUUID(ctx context.Context, puuid string) ([]LeagueEntryDTO, error) {
	u := fmt.Sprintf("%s/lol/league/v4/entries/by-puuid/%s", c.platformURL, url.PathEscape(puuid))
	var entries []LeagueEntryDTO
	return entries, c.doRequest(ctx, u, &entries)
}
