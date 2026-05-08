package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/argusxdr/argus/cmd/argus/tui/api"
	"github.com/argusxdr/argus/cmd/argus/tui/auth"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

// TestWSClient_AuthHeaderOnUpgrade verifies that the WebSocket Upgrade request
// carries Authorization: Bearer <token> in the header and the URL does NOT
// contain a ?token= query parameter. This is security constraint 2.
func TestWSClient_AuthHeaderOnUpgrade(t *testing.T) {
	var capturedAuthHeader string
	var capturedQuery string

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuthHeader = r.Header.Get("Authorization")
		capturedQuery = r.URL.RawQuery
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		// Immediately close from server side.
	}))
	defer srv.Close()

	// Convert http:// to ws://.
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	state := auth.NewAuthState()
	state.Set("ws-access-token", "ws-refresh", time.Now().Add(time.Hour), "user@example.com", "admin")

	authClient := auth.NewClient(srv.URL)
	client := api.NewClient(srv.URL, state, authClient)

	wsClient, err := client.Dial(context.Background(), wsURL+"/api/v1/signals/stream")
	if err == nil {
		wsClient.Close()
	}
	// We don't require err to be nil (server closes immediately) but the header
	// capture should have happened.

	// Token must be in Authorization header.
	assert.Equal(t, "Bearer ws-access-token", capturedAuthHeader,
		"Authorization header must carry the bearer token")

	// Token must NOT appear in the query string.
	assert.NotContains(t, capturedQuery, "token=",
		"token must never appear as a query parameter on the WS Upgrade URL")
}

// TestWSClient_Dial_ConnectionError verifies that Dial returns an error for an
// unreachable address (smoke test for error propagation).
func TestWSClient_Dial_ConnectionError(t *testing.T) {
	state := auth.NewAuthState()
	state.Set("token", "refresh", time.Now().Add(time.Hour), "user@example.com", "admin")
	authClient := auth.NewClient("http://localhost:19999") // nothing listening
	client := api.NewClient("http://localhost:19999", state, authClient)

	_, err := client.Dial(context.Background(), "ws://localhost:19999/stream")
	assert.Error(t, err, "Dial to unreachable host should return an error")
}
