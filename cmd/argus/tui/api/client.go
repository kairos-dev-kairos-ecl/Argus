package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/argusxdr/argus/cmd/argus/tui/auth"
)

// ErrUnauthenticated is returned by APIClient when a 401 is received and the
// subsequent token refresh also fails. The caller should route back to login.
var ErrUnauthenticated = errors.New("api: session expired — please log in again")

// Client is an HTTP client for the Argus backend with bearer token injection
// and automatic 401 refresh-and-retry logic.
type Client struct {
	baseURL    string
	httpClient *http.Client
	state      *auth.AuthState
	authClient *auth.Client
}

// NewClient creates an API client.
func NewClient(baseURL string, state *auth.AuthState, authClient *auth.Client) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		state:      state,
		authClient: authClient,
	}
}

// Get performs a GET request. The response body is JSON-decoded into out if
// out is non-nil.
func (c *Client) Get(ctx context.Context, path string, out any) error {
	return c.do(ctx, http.MethodGet, path, nil, out)
}

// Post performs a POST request with body serialized as JSON.
func (c *Client) Post(ctx context.Context, path string, body any, out any) error {
	return c.do(ctx, http.MethodPost, path, body, out)
}

// Delete performs a DELETE request.
func (c *Client) Delete(ctx context.Context, path string) error {
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// do executes the request with bearer token injection. On 401 it refreshes
// the token and retries exactly once. If the retry also returns 401 (or the
// refresh itself fails), state is cleared and ErrUnauthenticated is returned.
func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	resp, err := c.makeRequest(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// First attempt succeeded.
	if resp.StatusCode != http.StatusUnauthorized {
		return c.decodeResponse(resp, out)
	}

	// --- 401 path: try refresh once ---
	refreshErr := c.authClient.RefreshTokens(ctx, c.state)
	if refreshErr != nil {
		// Refresh failed — clear state and return ErrUnauthenticated.
		c.state.ClearOnLogout()
		return ErrUnauthenticated
	}

	// Close the first response body before retrying.
	resp.Body.Close()

	// Retry the original request with the new token.
	retryResp, err := c.makeRequest(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer retryResp.Body.Close()

	if retryResp.StatusCode == http.StatusUnauthorized {
		// Still 401 after refresh — clear state.
		c.state.ClearOnLogout()
		return ErrUnauthenticated
	}

	return c.decodeResponse(retryResp, out)
}

// makeRequest builds and sends a single HTTP request with the current bearer token.
// The token is injected from state.Bearer() at call time to pick up any refreshed token.
func (c *Client) makeRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("api: marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("api: build request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// SECURITY CONSTRAINT 1: token goes in Authorization header, NEVER in URL query params.
	bearer := c.state.Bearer()
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}

	return c.httpClient.Do(req)
}

// decodeResponse reads and optionally JSON-decodes the response body.
func (c *Client) decodeResponse(resp *http.Response, out any) error {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("api: read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{
			Status:  resp.StatusCode,
			Message: fmt.Sprintf("api error %d: %s", resp.StatusCode, string(data)),
		}
	}

	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("api: decode response: %w", err)
		}
	}
	return nil
}

// Dial creates a new WebSocket connection to the given wsURL.
// See ws.go for the WSClient implementation.
func (c *Client) Dial(ctx context.Context, wsURL string) (*WSClient, error) {
	return dialWS(ctx, wsURL, c.state)
}
