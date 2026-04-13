package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ocelik94/petrel-wings/internal/filesystem"
	"github.com/ocelik94/petrel-wings/internal/server"
)

// ServerHandler handles server and file management API endpoints.
type ServerHandler struct {
	Manager *server.Manager
}

// ListServers returns all servers.
func (h *ServerHandler) ListServers(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.Manager.List())
}

// GetServer returns a single server and current usage.
func (h *ServerHandler) GetServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	srv, ok := h.Manager.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}
	usage, err := srv.Usage(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"server": srv.Snapshot(),
		"usage":  usage,
	})
}

// CreateServer provisions a new server.
func (h *ServerHandler) CreateServer(w http.ResponseWriter, r *http.Request) {
	var req server.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	created, err := h.Manager.Create(ctx, req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			writeError(w, http.StatusGatewayTimeout, err.Error())
			return
		}
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// DeleteServer removes a server and all its files.
func (h *ServerHandler) DeleteServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.Manager.Delete(r.Context(), id); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "server not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PowerServer executes server power actions.
func (h *ServerHandler) PowerServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	srv, ok := h.Manager.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}
	var req struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	var err error
	switch req.Action {
	case "start":
		err = srv.Start(r.Context())
	case "stop":
		err = srv.Stop(r.Context())
	case "restart":
		err = srv.Restart(r.Context())
	case "kill":
		err = srv.Kill(r.Context())
	default:
		writeError(w, http.StatusBadRequest, "invalid power action")
		return
	}
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ListFiles lists files under the requested server path.
func (h *ServerHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	fs, ok := h.serverFS(w, r)
	if !ok {
		return
	}
	entries, err := fs.List(r.URL.Query().Get("path"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

// ReadFile reads file contents for a server.
func (h *ServerHandler) ReadFile(w http.ResponseWriter, r *http.Request) {
	fs, ok := h.serverFS(w, r)
	if !ok {
		return
	}
	content, err := fs.ReadFile(r.URL.Query().Get("path"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"content": string(content)})
}

// WriteFile writes file contents for a server.
func (h *ServerHandler) WriteFile(w http.ResponseWriter, r *http.Request) {
	fs, ok := h.serverFS(w, r)
	if !ok {
		return
	}
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	if err := fs.WriteFile(req.Path, []byte(req.Content)); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteFiles deletes files for a server.
func (h *ServerHandler) DeleteFiles(w http.ResponseWriter, r *http.Request) {
	fs, ok := h.serverFS(w, r)
	if !ok {
		return
	}
	var req struct {
		Paths []string `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := fs.DeleteFiles(req.Paths); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ServerHandler) serverFS(w http.ResponseWriter, r *http.Request) (*filesystem.Filesystem, bool) {
	id := chi.URLParam(r, "id")
	srv, ok := h.Manager.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "server not found")
		return nil, false
	}
	snapshot := srv.Snapshot()
	return filesystem.New(snapshot.DataPath), true
}

func writeJSON(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}
