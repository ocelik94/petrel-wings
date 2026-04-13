package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ocelik94/petrel-wings/internal/server"
)

// NewRouter builds the chi router and wires all handlers.
func NewRouter(token string, manager *server.Manager, logger *slog.Logger) http.Handler {
	r := chi.NewRouter()
	r.Use(RequestLogger(logger))

	auth := NewAuthMiddleware(token)
	serverHandler := &ServerHandler{Manager: manager}
	wsHandler := &WebSocketHandler{Manager: manager, Token: token}

	r.Get("/api/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	r.Group(func(protected chi.Router) {
		protected.Use(auth.Require)
		protected.Get("/api/servers", serverHandler.ListServers)
		protected.Get("/api/servers/{id}", serverHandler.GetServer)
		protected.Post("/api/servers", serverHandler.CreateServer)
		protected.Delete("/api/servers/{id}", serverHandler.DeleteServer)
		protected.Post("/api/servers/{id}/power", serverHandler.PowerServer)
		protected.Get("/api/servers/{id}/ws", wsHandler.Serve)
		protected.Get("/api/servers/{id}/files/list", serverHandler.ListFiles)
		protected.Get("/api/servers/{id}/files/contents", serverHandler.ReadFile)
		protected.Post("/api/servers/{id}/files/write", serverHandler.WriteFile)
		protected.Post("/api/servers/{id}/files/delete", serverHandler.DeleteFiles)
	})

	return r
}
