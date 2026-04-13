package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/ocelik94/petrel-wings/internal/server"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool { return true },
}

// WebSocketHandler streams server console logs and accepts console input.
type WebSocketHandler struct {
	Manager *server.Manager
	Token   string
}

// Serve handles the websocket console endpoint.
func (h *WebSocketHandler) Serve(w http.ResponseWriter, r *http.Request) {
	if !h.validToken(r) {
		writeError(w, http.StatusUnauthorized, "invalid token")
		return
	}
	id := chi.URLParam(r, "id")
	srv, ok := h.Manager.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	history := srv.Console().History()
	for _, line := range history {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(line)); err != nil {
			return
		}
	}

	sub := srv.Console().Subscribe()
	defer srv.Console().Unsubscribe(sub)

	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case line, ok := <-sub:
				if !ok {
					return
				}
				if err := conn.WriteMessage(websocket.TextMessage, []byte(line)); err != nil {
					return
				}
			case <-ticker.C:
				if err := conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(5*time.Second)); err != nil {
					return
				}
			}
		}
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			<-done
			return
		}
		if err := srv.WriteCommand(string(msg)); err != nil {
			_ = conn.WriteMessage(websocket.TextMessage, []byte("error: "+err.Error()))
		}
	}
}

func (h *WebSocketHandler) validToken(r *http.Request) bool {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return false
	}
	token := strings.TrimPrefix(header, "Bearer ")
	return len(token) == len(h.Token) && subtle.ConstantTimeCompare([]byte(token), []byte(h.Token)) == 1
}
