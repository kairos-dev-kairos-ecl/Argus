package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/argusxdr/argus/cmd/argus/tui/api"
	"github.com/argusxdr/argus/cmd/argus/tui/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAPIClient_AuthHeaderNotQueryParam verifies that the Authorization: Bearer
// token is sent in the HTTP header and NEVER as a ?token= query parameter.
// This is security constraint 1.
func TestAPIClient_AuthHeaderNotQueryParam(t *testing.T) {
	var capturedReq *http.Request

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	state := auth.NewAuthState()
	state.Set("my-access-token", "my-refresh", time.Now().Add(time.Hour), "user@example.com", "admin")

	authClient := auth.NewClient(srv.URL)
	client := api.NewClient(srv.URL, state, authClient)

	err := client.Get(context.Background(), "/api/v1/test", nil)
	require.NoError(t, err)

	// Token must be in Authorization header.
	authHeader := capturedReq.Header.Get("Authorization")
	assert.Equal(t, "Bearer my-access-token", authHeader)

	// Token must NOT be in the query string.
	assert.NotContains(t, capturedReq.URL.RawQuery, "token=",
		"token must never appear as a query parameter")
}

// TestAPIClient_401_RefreshAndRetry verifies that a 401 triggers exactly one
// refresh call followed by exactly one retry of the original request.
func TestAPIClient_401_RefreshAndRetry(t *testing.T) {
	var requestCount int32
	var refreshCount int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/refresh":
			atomic.AddInt32(&refreshCount, 1)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "new-access",
				"refresh_token": "new-refresh",
				"user": map[string]any{
					"email": "user@example.com",
					"role":  "admin",
				},
			})
		default:
			n := atomic.AddInt32(&requestCount, 1)
			if n == 1 {
				w.WriteHeader(http.StatusUnauthorized)
			} else {
				w.WriteHeader(http.StatusOK)
			}
		}
	}))
	defer srv.Close()

	state := auth.NewAuthState()
	state.Set("stale-token", "my-refresh", time.Now().Add(time.Hour), "user@example.com", "admin")

	authClient := auth.NewClient(srv.URL)
	client := api.NewClient(srv.URL, state, authClient)

	err := client.Get(context.Background(), "/api/v1/test", nil)
	require.NoError(t, err)

	assert.Equal(t, int32(2), requestCount, "original request should be made exactly twice (initial + retry)")
	assert.Equal(t, int32(1), refreshCount, "refresh should be called exactly once")
}

// TestAPIClient_401_RefreshFail_ClearsState verifies that when a 401 is received
// and the refresh attempt itself fails, the state is cleared and ErrUnauthenticated
// is returned. This is security constraint 3.
func TestAPIClient_401_RefreshFail_ClearsState(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// All requests return 401, including the refresh endpoint.
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	state := auth.NewAuthState()
	state.Set("stale-token", "stale-refresh", time.Now().Add(time.Hour), "user@example.com", "admin")

	authClient := auth.NewClient(srv.URL)
	client := api.NewClient(srv.URL, state, authClient)

	err := client.Get(context.Background(), "/api/v1/test", nil)

	// Must return ErrUnauthenticated.
	assert.ErrorIs(t, err, api.ErrUnauthenticated,
		"should return ErrUnauthenticated when refresh fails")
	// State must be cleared.
	assert.Equal(t, "", state.Bearer(),
		"auth state must be cleared when refresh fails (security constraint 3)")
}
