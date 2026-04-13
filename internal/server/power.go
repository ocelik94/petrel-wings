package server

import (
	"bufio"
	"context"
	"fmt"
	"time"

	wdocker "github.com/ocelik94/petrel-wings/internal/docker"
)

const (
	waitContainerTimeout = 8 * time.Second
	stopContainerTimeout = 10 * time.Second
)

// Start starts the server container and attaches console streams.
func (s *Server) Start(ctx context.Context) error {
	state := s.State()
	if state != StateStopped && state != StateError {
		return fmt.Errorf("server cannot start from state %q", state)
	}
	if err := s.SetState(StateStarting); err != nil {
		return err
	}

	s.mu.Lock()
	containerID := s.ContainerID
	s.mu.Unlock()
	if containerID == "" {
		id, err := s.docker.CreateContainer(ctx, wdocker.ContainerSpec{
			Name:           "petrel-" + s.ID,
			Image:          s.Image,
			Cmd:            []string{"sh", "-lc", s.Startup},
			Env:            s.envSlice(),
			DataPath:       s.DataPath,
			MemoryMB:       s.Limits.MemoryMB,
			CPUPercent:     s.Limits.CPU,
			ExposedTCPPort: s.Ports,
			PortBindings:   map[string]string{},
			Network:        s.network,
		})
		if err != nil {
			s.ForceState(StateError)
			return fmt.Errorf("creating container: %w", err)
		}
		s.mu.Lock()
		s.ContainerID = id
		containerID = id
		s.mu.Unlock()
	}

	if err := s.docker.StartContainer(ctx, containerID); err != nil {
		s.ForceState(StateError)
		return fmt.Errorf("starting container: %w", err)
	}

	attached, closeFn, err := s.docker.AttachContainer(ctx, containerID)
	if err != nil {
		s.ForceState(StateError)
		return fmt.Errorf("attaching to container: %w", err)
	}

	s.mu.Lock()
	s.stdin = attached.Stdin
	s.attachEnd = closeFn
	s.mu.Unlock()

	go func() {
		scanner := bufio.NewScanner(attached.Stdout)
		for scanner.Scan() {
			s.console.Append(scanner.Text())
		}
	}()

	s.ForceState(StateRunning)
	return nil
}

// Stop gracefully stops a running server.
func (s *Server) Stop(ctx context.Context) error {
	if s.State() != StateRunning {
		return nil
	}
	if err := s.SetState(StateStopping); err != nil {
		return err
	}

	_ = s.WriteCommand("stop")

	s.mu.RLock()
	containerID := s.ContainerID
	s.mu.RUnlock()
	waitCtx, waitCancel := context.WithTimeout(ctx, waitContainerTimeout)
	_, waitErr := s.docker.WaitContainer(waitCtx, containerID)
	waitCancel()

	if waitErr != nil {
		stopCtx, cancel := context.WithTimeout(ctx, stopContainerTimeout)
		defer cancel()
		if err := s.docker.StopContainer(stopCtx, containerID, stopContainerTimeout); err != nil {
			s.ForceState(StateError)
			return fmt.Errorf("stopping container: %w", err)
		}
	}

	s.mu.Lock()
	if s.attachEnd != nil {
		_ = s.attachEnd()
	}
	s.stdin = nil
	s.attachEnd = nil
	s.mu.Unlock()

	s.ForceState(StateStopped)
	return nil
}

// Restart restarts the server container.
func (s *Server) Restart(ctx context.Context) error {
	if err := s.Stop(ctx); err != nil {
		return err
	}
	return s.Start(ctx)
}

// Kill force-removes the server container.
func (s *Server) Kill(ctx context.Context) error {
	s.mu.RLock()
	containerID := s.ContainerID
	s.mu.RUnlock()
	if containerID == "" {
		return nil
	}
	if err := s.docker.RemoveContainer(ctx, containerID, true); err != nil {
		return fmt.Errorf("force removing container: %w", err)
	}
	s.mu.Lock()
	if s.attachEnd != nil {
		_ = s.attachEnd()
	}
	s.stdin = nil
	s.attachEnd = nil
	s.ContainerID = ""
	s.mu.Unlock()
	s.ForceState(StateStopped)
	s.console.Append("container killed")
	return nil
}
