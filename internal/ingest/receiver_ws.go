package ingest

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	wsPingInterval = 30 * time.Second
	wsWriteTimeout = 5 * time.Second
	wsReadTimeout  = 60 * time.Second // must be > 2× ping interval
)

// WebSocketHandler handles WebSocket upgrades and streams signals.
// It fans out ArgusSignal messages to all connected WebSocket clients.
type WebSocketHandler struct {
	broadcaster *SignalBroadcaster
	log         *zap.Logger
	upgrader    websocket.Upgrader
}

// NewWebSocketHandler creates a new WebSocket handler.
func NewWebSocketHandler(b *SignalBroadcaster, log *zap.Logger) *WebSocketHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &WebSocketHandler{
		broadcaster: b,
		log:         log,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// In production, restrict to same-origin. For dev, allow all.
				return true
			},
		},
	}
}

// RegisterRoutes registers WebSocket routes.
func (h *WebSocketHandler) RegisterRoutes(mux *chi.Mux) {
	mux.Get("/v1/signals/stream", h.handleStream)
}

// handleStream upgrades to WebSocket and streams signals from the broadcaster.
// Each signal is marshaled to JSON and sent as a WebSocket text message.
// A 30-second ping ticker detects silent disconnections (NAT timeout, dead TCP).
func (h *WebSocketHandler) handleStream(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Warn("websocket upgrade failed", zap.Error(err))
		return
	}
	defer conn.Close()

	// Subscribe immediately after upgrade so signals published after dial are not missed.
	sigChan := h.broadcaster.Subscribe()
	defer h.broadcaster.Unsubscribe(sigChan)

	// Pong handler resets the read deadline on every pong received.
	conn.SetReadDeadline(time.Now().Add(wsReadTimeout))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(wsReadTimeout))
		return nil
	})

	pingTicker := time.NewTicker(wsPingInterval)
	defer pingTicker.Stop()

	h.log.Debug("websocket client connected", zap.String("remote_addr", conn.RemoteAddr().String()))

	for {
		select {
		case sig, ok := <-sigChan:
			if !ok {
				return // broadcaster closed
			}
			data, err := json.Marshal(sig)
			if err != nil {
				h.log.Warn("failed to marshal signal", zap.Error(err))
				continue
			}
			conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				h.log.Debug("websocket write error (client disconnected)", zap.Error(err))
				return
			}

		case <-pingTicker.C:
			conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				h.log.Debug("websocket ping failed (client disconnected)", zap.Error(err))
				return
			}

		case <-r.Context().Done():
			return
		}
	}
}
