package docker

import (
	"context"
	"fmt"

	dockerclient "github.com/docker/docker/client"
)

// Client wraps the Docker SDK client.
type Client struct {
	cli *dockerclient.Client
}

// NewClient creates a Docker client with API version negotiation.
func NewClient(socket string) (*Client, error) {
	opts := []dockerclient.Opt{dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation()}
	if socket != "" {
		opts = append(opts, dockerclient.WithHost("unix://"+socket))
	}
	cli, err := dockerclient.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}
	return &Client{cli: cli}, nil
}

// Close closes the Docker client.
func (c *Client) Close() error {
	return c.cli.Close()
}

// Ping verifies Docker API connectivity.
func (c *Client) Ping(ctx context.Context) error {
	if _, err := c.cli.Ping(ctx); err != nil {
		return fmt.Errorf("pinging docker: %w", err)
	}
	return nil
}
