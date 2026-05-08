package api

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/argusxdr/argus/cmd/argus/tui/auth"
	"github.com/gorilla/websocket"
)

const (
	wsReconnectInitial  = 500 * time.Millisecond
	wsReconnectCap      = 30 * time.Second
	wsReconnectMaxTries = 5
	wsMsgBufSize        = 64
)

// WSClient manages a WebSocket connection to the Argus signal stream.
// Authentication is performed via the Authorization: Bearer header on the
// Upgrade request — NEVER via a ?token= query parameter (security constraint 2).
type WSClient struct {
	conn   *websocket.Conn
	msgs   chan WSMsg
	done   chan struct{}
	state  *auth.AuthState
	wsURL  string
}

// dialWS dials a WebSocket connection to wsURL using a bearer token in the
// Upgrade request headers.
//
// SECURITY CONSTRAINT 2: The Authorization header is used for the token.
// The wsURL MUST NOT contain a `token=` query parameter.
func dialWS(ctx context.Context, wsURL string, state *auth.AuthState) (*WSClient, error) {
	// Build the Upgrade request headers with the bearer token.
	// NEVER append ?token= to the URL — server logs would capture it.
	headers := http.Header{}
	bearer := state.Bearer()
	if bearer != "" {
		headers.Set("Authorization", "Bearer "+bearer)
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		return nil, fmt.Errorf("ws: dial %s: %w", wsURL, err)
	}

	client := &WSClient{
		conn:  conn,
		msgs:  make(chan WSMsg, wsMsgBufSize),
		done:  make(chan struct{}),
		state: state,
		wsURL: wsURL,
	}

	go client.readLoop()

	return client, nil
}

// Messages returns the channel of decoded WebSocket messages.
// The channel is closed when the connection is permanently closed.
func (c *WSClient) Messages() <-chan WSMsg {
	return c.msgs
}

// Close shuts down the WebSocket connection and drains the message channel.
func (c *WSClient) Close() {
	close(c.done)
	if c.conn != nil {
		c.conn.Close()
	}
}

// readLoop reads messages from the WebSocket connection in a goroutine.
// It reconnects with exponential backoff (initial 500ms, cap 30s, ±20% jitter)
// for up to wsReconnectMaxTries attempts before giving up.
func (c *WSClient) readLoop() {
	defer close(c.msgs)

	for {
		select {
		case <-c.done:
			return
		default:
		}

		_, data, err := c.conn.ReadMessage()
		if err != nil {
			// Connection error — attempt reconnect.
			if !c.reconnect() {
				return
			}
			continue
		}

		var msg WSMsg
		if jsonErr := json.Unmarshal(data, &msg); jsonErr != nil {
			// Skip malformed messages.
			continue
		}

		select {
		case c.msgs <- msg:
		case <-c.done:
			return
		}
	}
}

// reconnect attempts to re-establish the WebSocket connection with exponential
// backoff. Returns false if the maximum retry count is exceeded or done is closed.
func (c *WSClient) reconnect() bool {
	delay := wsReconnectInitial

	for attempt := 0; attempt < wsReconnectMaxTries; attempt++ {
		select {
		case <-c.done:
			return false
		default:
		}

		// Add ±20% jitter.
		jitter := time.Duration(float64(delay) * (0.8 + 0.4*rand.Float64())) //nolint:gosec
		time.Sleep(jitter)

		// Re-fetch the bearer token (it may have been refreshed).
		headers := http.Header{}
		bearer := c.state.Bearer()
		if bearer != "" {
			headers.Set("Authorization", "Bearer "+bearer)
		}

		dialer := websocket.Dialer{
			HandshakeTimeout: 10 * time.Second,
		}

		conn, _, err := dialer.DialContext(context.Background(), c.wsURL, headers)
		if err == nil {
			c.conn = conn
			return true
		}

		// Exponential backoff, capped at wsReconnectCap.
		delay *= 2
		if delay > wsReconnectCap {
			delay = wsReconnectCap
		}
	}

	return false
}
