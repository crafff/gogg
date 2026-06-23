package riotapi

import (
	"context"
	"net/http"
	"regexp"
	"time"
)

// VersionEntry is a single patch version with its release date.
type VersionEntry struct {
	Version      string
	PatchStartAt time.Time
}

var versionPattern = regexp.MustCompile(`^\d+\.\d+$`)

// GetAllVersions fetches every patch version and its start date from CommunityDragon.
// Entries already in the database can be skipped by the caller (ON CONFLICT DO NOTHING).
func (c *Client) GetAllVersions(ctx context.Context) ([]VersionEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://raw.communitydragon.org/json/", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var raw []struct {
		Name  string `json:"name"`
		Type  string `json:"type"`
		Mtime string `json:"mtime"`
	}
	if err := decodeJSON(resp.Body, &raw); err != nil {
		return nil, err
	}

	var entries []VersionEntry
	for _, item := range raw {
		if item.Type != "directory" || !versionPattern.MatchString(item.Name) {
			continue
		}
		t, err := time.Parse(time.RFC1123, item.Mtime)
		if err != nil {
			continue
		}
		entries = append(entries, VersionEntry{Version: item.Name, PatchStartAt: t})
	}
	return entries, nil
}
