package ingest_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/ingest"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestWebSocketHandler_ReceivesSignals(t *testing.T) {
	broadcaster := ingest.NewSignalBroadcaster()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go broadcaster.Run(ctx)

	mux := chi.NewRouter()
	wsHandler := ingest.NewWebSocketHandler(broadcaster, zap.NewNop())
	wsHandler.RegisterRoutes(mux)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/signals/stream"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	// Publish a signal
	sig := &v1.ArgusSignal{SignalId: "ws-test-001", TraceId: "trace-ws"}
	broadcaster.Publish(sig)

	// Should receive within 1 second
	conn.SetReadDeadline(time.Now().Add(time.Second))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Contains(t, string(msg), "ws-test-001")
}

func TestWebSocketHandler_MultipleClients(t *testing.T) {
	broadcaster := ingest.NewSignalBroadcaster()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go broadcaster.Run(ctx)

	mux := chi.NewRouter()
	wsHandler := ingest.NewWebSocketHandler(broadcaster, zap.NewNop())
	wsHandler.RegisterRoutes(mux)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/signals/stream"

	// Connect two clients
	conn1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn1.Close()

	conn2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn2.Close()

	// Publish a signal
	sig := &v1.ArgusSignal{SignalId: "multi-client-test"}
	broadcaster.Publish(sig)

	// Both clients should receive it
	for _, conn := range []*websocket.Conn{conn1, conn2} {
		conn.SetReadDeadline(time.Now().Add(time.Second))
		_, msg, err := conn.ReadMessage()
		require.NoError(t, err)
		assert.Contains(t, string(msg), "multi-client-test")
	}
}
