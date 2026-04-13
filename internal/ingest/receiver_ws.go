package ingest

import (
	"encoding/json"
	"net/http"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
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
// Argument r carries request context; response w receives the upgrade.
func (h *WebSocketHandler) handleStream(w http.ResponseWriter, r *http.Request) {
	// Ensure v1 import is used: we receive v1.ArgusSignal pointers from broadcaster
	var _ *v1.ArgusSignal

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Warn("websocket upgrade failed", zap.Error(err))
		return
	}
	defer conn.Close()

	// Subscribe to broadcaster
	sigChan := h.broadcaster.Subscribe()
	defer h.broadcaster.Unsubscribe(sigChan)

	h.log.Debug("websocket client connected", zap.String("remote_addr", conn.RemoteAddr().String()))

	// Stream signals to the client (v1.ArgusSignal via channel)
	for sig := range sigChan {
		// Encode signal as JSON
		data, err := json.Marshal(sig)
		if err != nil {
			h.log.Warn("failed to marshal signal", zap.Error(err))
			continue
		}

		// Send to WebSocket
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			h.log.Debug("websocket write error", zap.Error(err))
			break
		}
	}

	h.log.Debug("websocket client disconnected", zap.String("remote_addr", conn.RemoteAddr().String()))
}
