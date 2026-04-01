package riot

import (
	"context"
)

type VersionResponse []struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Mtime string `json:"mtime"`
}

func (c *Client) GetVersions(ctx context.Context) (*VersionResponse, error) {
	url := "https://raw.communitydragon.org/json/"
	
	var versions VersionResponse
	err := c.doRequest(ctx, "GET", url, &versions)
	if err != nil {
		return nil, err
	}
	
	return &versions, nil
}