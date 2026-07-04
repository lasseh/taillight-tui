package client

import (
	"context"
	"fmt"
)

// NetlogHosts fetches the list of known netlog hostnames.
func (c *Client) NetlogHosts(ctx context.Context) ([]string, error) {
	var resp ItemResponse[[]string]
	if err := c.get(ctx, "/api/v1/netlog/meta/hosts", &resp); err != nil {
		return nil, fmt.Errorf("netlog hosts: %w", err)
	}
	return resp.Data, nil
}

// NetlogPrograms fetches the list of known netlog program names.
func (c *Client) NetlogPrograms(ctx context.Context) ([]string, error) {
	var resp ItemResponse[[]string]
	if err := c.get(ctx, "/api/v1/netlog/meta/programs", &resp); err != nil {
		return nil, fmt.Errorf("netlog programs: %w", err)
	}
	return resp.Data, nil
}
