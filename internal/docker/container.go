package docker

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
)

// ContainerSpec contains container runtime configuration.
type ContainerSpec struct {
	Name           string
	Image          string
	Cmd            []string
	Env            []string
	DataPath       string
	MemoryMB       int64
	CPUPercent     int64
	ExposedTCPPort []string
	PortBindings   map[string]string
	Network        string
}

// PullImage pulls an image from the registry.
func (c *Client) PullImage(ctx context.Context, imageRef string) error {
	reader, err := c.cli.ImagePull(ctx, imageRef, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("pulling image: %w", err)
	}
	defer reader.Close()
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return fmt.Errorf("draining image pull response: %w", err)
	}
	return nil
}

// CreateContainer creates a container from the provided spec.
func (c *Client) CreateContainer(ctx context.Context, spec ContainerSpec) (string, error) {
	exposed := nat.PortSet{}
	bindings := nat.PortMap{}
	for _, port := range spec.ExposedTCPPort {
		nport := nat.Port(port + "/tcp")
		exposed[nport] = struct{}{}
		if hostPort, ok := spec.PortBindings[port]; ok && hostPort != "" {
			bindings[nport] = []nat.PortBinding{{HostPort: hostPort}}
		}
	}

	memBytes := spec.MemoryMB * 1024 * 1024
	cpuQuota := int64(0)
	if spec.CPUPercent > 0 {
		cpuQuota = spec.CPUPercent * 1000
	}

	resp, err := c.cli.ContainerCreate(
		ctx,
		&container.Config{
			Image:        spec.Image,
			Cmd:          spec.Cmd,
			Env:          spec.Env,
			ExposedPorts: exposed,
			Tty:          true,
			OpenStdin:    true,
			AttachStdout: true,
			AttachStderr: true,
		},
		&container.HostConfig{
			Binds: []string{spec.DataPath + ":/home/container"},
			Resources: container.Resources{
				Memory:   memBytes,
				CPUQuota: cpuQuota,
				CPUPeriod: func() int64 {
					if cpuQuota > 0 {
						return 100000
					}
					return 0
				}(),
			},
			PortBindings: bindings,
			RestartPolicy: container.RestartPolicy{
				Name: "unless-stopped",
			},
		},
		&network.NetworkingConfig{},
		nil,
		spec.Name,
	)
	if err != nil {
		return "", fmt.Errorf("creating container: %w", err)
	}
	if spec.Network != "" {
		if err := c.cli.NetworkConnect(ctx, spec.Network, resp.ID, nil); err != nil {
			return "", fmt.Errorf("connecting container to network: %w", err)
		}
	}
	return resp.ID, nil
}

// StartContainer starts a container.
func (c *Client) StartContainer(ctx context.Context, containerID string) error {
	if err := c.cli.ContainerStart(ctx, containerID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("starting container: %w", err)
	}
	return nil
}

// StopContainer gracefully stops a container.
func (c *Client) StopContainer(ctx context.Context, containerID string, timeout time.Duration) error {
	if err := c.cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: durationPtr(int(timeout.Seconds()))}); err != nil {
		return fmt.Errorf("stopping container: %w", err)
	}
	return nil
}

func durationPtr(v int) *int {
	return &v
}

// RemoveContainer removes a container.
func (c *Client) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	if err := c.cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{Force: force}); err != nil {
		return fmt.Errorf("removing container: %w", err)
	}
	return nil
}

// AttachedIO represents attached container streams.
type AttachedIO struct {
	Stdin  io.WriteCloser
	Stdout io.Reader
}

// AttachContainer attaches to a running container.
func (c *Client) AttachContainer(ctx context.Context, containerID string) (*AttachedIO, func() error, error) {
	resp, err := c.cli.ContainerAttach(ctx, containerID, types.ContainerAttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
		Logs:   true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("attaching container: %w", err)
	}

	reader, writer := io.Pipe()
	go func() {
		_, copyErr := stdcopy.StdCopy(writer, writer, resp.Reader)
		_ = writer.CloseWithError(copyErr)
	}()

	return &AttachedIO{Stdin: resp.Conn, Stdout: reader}, func() error {
		resp.Close()
		return nil
	}, nil
}

// WaitContainer waits until a container exits.
func (c *Client) WaitContainer(ctx context.Context, containerID string) (int64, error) {
	statusCh, errCh := c.cli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return 0, fmt.Errorf("waiting for container: %w", err)
		}
	case status := <-statusCh:
		return status.StatusCode, nil
	}
	return 0, nil
}
