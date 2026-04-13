package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	wdocker "github.com/ocelik94/petrel-wings/internal/docker"
)

// State describes a server lifecycle state.
type State string

const (
	// StateInstalling means setup scripts are running.
	StateInstalling State = "installing"
	// StateStopped means the server is stopped.
	StateStopped State = "stopped"
	// StateStarting means startup is in progress.
	StateStarting State = "starting"
	// StateRunning means the server is online.
	StateRunning State = "running"
	// StateStopping means stop action is in progress.
	StateStopping State = "stopping"
	// StateError means the server is in an error state.
	StateError State = "error"
)

var validTransitions = map[State]map[State]struct{}{
	StateInstalling: {StateStopped: {}, StateError: {}},
	StateStopped:    {StateStarting: {}},
	StateStarting:   {StateRunning: {}, StateError: {}},
	StateRunning:    {StateStopping: {}},
	StateStopping:   {StateStopped: {}, StateError: {}},
	StateError:      {StateStarting: {}},
}

// Limits stores resource limits.
type Limits struct {
	MemoryMB int64 `json:"memory_mb" yaml:"memory_mb"`
	DiskMB   int64 `json:"disk_mb" yaml:"disk_mb"`
	CPU      int64 `json:"cpu_percent" yaml:"cpu_percent"`
}

// Server stores server metadata and runtime state.
type Server struct {
	mu sync.RWMutex

	ID      string            `json:"id" yaml:"id"`
	Name    string            `json:"name" yaml:"name"`
	Image   string            `json:"image" yaml:"image"`
	Startup string            `json:"startup" yaml:"startup"`
	Env     map[string]string `json:"env" yaml:"env"`
	Limits  Limits            `json:"limits" yaml:"limits"`
	Ports   []string          `json:"ports" yaml:"ports"`

	state       State
	ContainerID string `json:"container_id" yaml:"container_id"`
	DataPath    string `json:"data_path" yaml:"data_path"`

	console   *Console
	docker    *wdocker.Client
	network   string
	stdin     io.WriteCloser
	attachEnd func() error
}

// NewServer creates a server instance in stopped state.
func NewServer(dc *wdocker.Client, network string, data Server) *Server {
	if data.Env == nil {
		data.Env = map[string]string{}
	}
	if data.console == nil {
		data.console = NewConsole()
	}
	data.docker = dc
	data.network = network
	if data.state == "" {
		data.state = StateStopped
	}
	return &data
}

// State returns the current server state.
func (s *Server) State() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// SetState transitions state if valid.
func (s *Server) SetState(next State) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == next {
		return nil
	}
	nextStates, ok := validTransitions[s.state]
	if !ok {
		return fmt.Errorf("state %q has no transitions", s.state)
	}
	if _, ok := nextStates[next]; !ok {
		return fmt.Errorf("invalid transition %q -> %q", s.state, next)
	}
	s.state = next
	return nil
}

// ForceState updates server state without transition checks.
func (s *Server) ForceState(next State) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = next
}

// Snapshot returns a detached copy of the server.
func (s *Server) Snapshot() Server {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := *s
	cp.mu = sync.RWMutex{}
	cp.console = nil
	cp.docker = nil
	cp.stdin = nil
	cp.attachEnd = nil
	return cp
}

// Console returns the console broadcaster.
func (s *Server) Console() *Console {
	return s.console
}

// WriteCommand writes a command to the container stdin.
func (s *Server) WriteCommand(command string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.stdin == nil {
		return errors.New("server stdin is not attached")
	}
	if _, err := io.WriteString(s.stdin, command+"\n"); err != nil {
		return fmt.Errorf("writing command to stdin: %w", err)
	}
	return nil
}

func (s *Server) envSlice() []string {
	out := make([]string, 0, len(s.Env))
	for k, v := range s.Env {
		out = append(out, k+"="+v)
	}
	return out
}

// Usage returns current resource usage if container is running.
func (s *Server) Usage(ctx context.Context) (wdocker.ResourceUsage, error) {
	s.mu.RLock()
	containerID := s.ContainerID
	s.mu.RUnlock()
	if containerID == "" {
		return wdocker.ResourceUsage{}, nil
	}
	return s.docker.GetStats(ctx, containerID)
}
