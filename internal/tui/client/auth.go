package client

import (
	"context"
)

// Health checks connectivity to the API server.
func (c *Client) Health(ctx context.Context) error {
	return c.get(ctx, "/health", nil)
}
