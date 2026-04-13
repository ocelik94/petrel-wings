package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"

	wdocker "github.com/ocelik94/petrel-wings/internal/docker"
	"gopkg.in/yaml.v3"
)

// Manager manages lifecycle and lookup of all servers on a node.
type Manager struct {
	mu       sync.RWMutex
	servers  map[string]*Server
	dataPath string
	docker   *wdocker.Client
	network  string
}

// CreateRequest describes a new server provisioning request.
type CreateRequest struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Image   string            `json:"image"`
	Startup string            `json:"startup"`
	Env     map[string]string `json:"env"`
	Limits  Limits            `json:"limits"`
	Ports   []string          `json:"ports"`
}

// NewManager creates a server manager.
func NewManager(dataPath string, dc *wdocker.Client, network string) *Manager {
	return &Manager{
		servers:  map[string]*Server{},
		dataPath: dataPath,
		docker:   dc,
		network:  network,
	}
}

// Initialize scans persisted servers and loads them into memory.
func (m *Manager) Initialize(ctx context.Context) error {
	if err := os.MkdirAll(m.baseServersPath(), 0o755); err != nil {
		return fmt.Errorf("creating servers data path: %w", err)
	}

	entries, err := os.ReadDir(m.baseServersPath())
	if err != nil {
		return fmt.Errorf("reading servers data path: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		configPath := filepath.Join(m.baseServersPath(), entry.Name(), "server.yml")
		content, err := os.ReadFile(configPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("reading server config %q: %w", configPath, err)
		}
		var data Server
		if err := yaml.Unmarshal(content, &data); err != nil {
			return fmt.Errorf("unmarshalling server config %q: %w", configPath, err)
		}
		if data.ID == "" {
			data.ID = entry.Name()
		}
		data.DataPath = filepath.Join(m.baseServersPath(), data.ID, "data")
		srv := NewServer(m.docker, m.network, data)
		m.mu.Lock()
		m.servers[srv.ID] = srv
		m.mu.Unlock()
	}
	return nil
}

// Create provisions and registers a new server.
func (m *Manager) Create(ctx context.Context, req CreateRequest) (Server, error) {
	if req.Name == "" || req.Image == "" || req.Startup == "" {
		return Server{}, errors.New("name, image and startup are required")
	}
	if req.ID == "" {
		req.ID = generateID()
	}

	m.mu.RLock()
	_, exists := m.servers[req.ID]
	m.mu.RUnlock()
	if exists {
		return Server{}, fmt.Errorf("server %q already exists", req.ID)
	}

	serverPath := filepath.Join(m.baseServersPath(), req.ID)
	dataPath := filepath.Join(serverPath, "data")
	if err := os.MkdirAll(dataPath, 0o755); err != nil {
		return Server{}, fmt.Errorf("creating server data directory: %w", err)
	}

	srv := NewServer(m.docker, m.network, Server{
		ID:       req.ID,
		Name:     req.Name,
		Image:    req.Image,
		Startup:  req.Startup,
		Env:      req.Env,
		Limits:   req.Limits,
		Ports:    req.Ports,
		state:    StateStopped,
		DataPath: dataPath,
	})

	if err := srv.Install(ctx); err != nil {
		return Server{}, fmt.Errorf("installing server: %w", err)
	}
	if err := m.persist(srv); err != nil {
		return Server{}, err
	}

	m.mu.Lock()
	m.servers[srv.ID] = srv
	m.mu.Unlock()
	return srv.Snapshot(), nil
}

// Get fetches a server by id.
func (m *Manager) Get(id string) (*Server, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	srv, ok := m.servers[id]
	return srv, ok
}

// List returns all servers sorted by ID.
func (m *Manager) List() []Server {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Server, 0, len(m.servers))
	for _, srv := range m.servers {
		out = append(out, srv.Snapshot())
	}
	slices.SortFunc(out, func(a, b Server) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	return out
}

// Delete removes a server and its data.
func (m *Manager) Delete(ctx context.Context, id string) error {
	m.mu.RLock()
	srv, ok := m.servers[id]
	m.mu.RUnlock()
	if !ok {
		return os.ErrNotExist
	}

	if err := srv.Kill(ctx); err != nil {
		return err
	}
	serverPath := filepath.Join(m.baseServersPath(), id)
	if err := os.RemoveAll(serverPath); err != nil {
		return fmt.Errorf("deleting server data directory: %w", err)
	}

	m.mu.Lock()
	delete(m.servers, id)
	m.mu.Unlock()
	return nil
}

// Shutdown gracefully stops all running servers.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.RLock()
	servers := make([]*Server, 0, len(m.servers))
	for _, srv := range m.servers {
		servers = append(servers, srv)
	}
	m.mu.RUnlock()

	var errs []error
	for _, srv := range servers {
		if err := srv.Stop(ctx); err != nil {
			errs = append(errs, fmt.Errorf("stopping %s: %w", srv.ID, err))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (m *Manager) persist(srv *Server) error {
	serverPath := filepath.Join(m.baseServersPath(), srv.ID)
	if err := os.MkdirAll(serverPath, 0o755); err != nil {
		return fmt.Errorf("creating server directory: %w", err)
	}
	data := srv.Snapshot()
	content, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshalling server config: %w", err)
	}
	if err := os.WriteFile(filepath.Join(serverPath, "server.yml"), content, 0o644); err != nil {
		return fmt.Errorf("writing server config: %w", err)
	}
	return nil
}

func (m *Manager) baseServersPath() string {
	return filepath.Join(m.dataPath, "servers")
}

func generateID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "srv-fallback"
	}
	return hex.EncodeToString(buf)
}
