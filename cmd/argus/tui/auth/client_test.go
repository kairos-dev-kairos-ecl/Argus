package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/argusxdr/argus/cmd/argus/tui/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuthState_ClearOnLogout_ZeroesFields verifies that ClearOnLogout zeros every
// auth field, satisfying security constraint 3.
func TestAuthState_ClearOnLogout_ZeroesFields(t *testing.T) {
	s := auth.NewAuthState()
	s.Set("access-token-xyz", "refresh-token-abc", time.Now().Add(time.Hour), "user@example.com", "admin")

	assert.Equal(t, "access-token-xyz", s.Bearer())

	s.ClearOnLogout()

	assert.Equal(t, "", s.Bearer(), "Bearer should be empty after ClearOnLogout")
	assert.Equal(t, "", s.RefreshToken(), "RefreshToken should be empty after ClearOnLogout")
	assert.True(t, s.ExpiresAt().IsZero(), "ExpiresAt should be zero after ClearOnLogout")
	assert.Equal(t, "", s.Email(), "Email should be empty after ClearOnLogout")
	assert.Equal(t, "", s.Role(), "Role should be empty after ClearOnLogout")
}

// TestAuthClient_Login_Success verifies a successful login populates AuthState.
func TestAuthClient_Login_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/auth/login", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "test-access",
			"refresh_token": "test-refresh",
			"user": map[string]any{
				"email": "admin@argus.io",
				"role":  "admin",
			},
		})
	}))
	defer srv.Close()

	client := auth.NewClient(srv.URL)
	state, mfaRequired, err := client.Login(context.Background(), "admin@argus.io", "password")

	require.NoError(t, err)
	assert.False(t, mfaRequired)
	assert.Equal(t, "test-access", state.Bearer())
	assert.Equal(t, "test-refresh", state.RefreshToken())
	assert.Equal(t, "admin@argus.io", state.Email())
	assert.Equal(t, "admin", state.Role())
}

// TestAuthClient_Login_MFARequired verifies that mfa_required: true returns
// mfaRequired=true with no finalized access token.
func TestAuthClient_Login_MFARequired(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"mfa_required": true,
			"mfa_token":    "temp-mfa-token",
		})
	}))
	defer srv.Close()

	client := auth.NewClient(srv.URL)
	state, mfaRequired, err := client.Login(context.Background(), "admin@argus.io", "password")

	require.NoError(t, err)
	assert.True(t, mfaRequired)
	// Access token should not be populated yet.
	assert.Equal(t, "", state.Bearer(), "access token should be empty when MFA required")
	// MFA token stored in RefreshToken field as scratch space.
	assert.NotEmpty(t, state.RefreshToken(), "mfa_token should be stored in RefreshToken scratch")
}

// TestAuthClient_RefreshTokens verifies that RefreshTokens updates the auth state.
func TestAuthClient_RefreshTokens(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/refresh" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "new-access",
				"refresh_token": "new-refresh",
				"user": map[string]any{
					"email": "admin@argus.io",
					"role":  "admin",
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	state := auth.NewAuthState()
	state.Set("old-access", "old-refresh", time.Now().Add(time.Hour), "admin@argus.io", "admin")

	client := auth.NewClient(srv.URL)
	err := client.RefreshTokens(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, "new-access", state.Bearer())
	assert.Equal(t, "new-refresh", state.RefreshToken())
}

// TestAuthClient_Logout_ClearsStateOnSuccess verifies that Logout clears the
// auth state after a successful POST.
func TestAuthClient_Logout_ClearsStateOnSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/auth/logout", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	state := auth.NewAuthState()
	state.Set("access", "refresh", time.Now().Add(time.Hour), "user@example.com", "admin")

	client := auth.NewClient(srv.URL)
	err := client.Logout(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, "", state.Bearer(), "state should be cleared after logout")
}

// TestAuthClient_Logout_ClearsStateOnError verifies that Logout clears state
// even when the HTTP call fails (security: always clear).
func TestAuthClient_Logout_ClearsStateOnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	state := auth.NewAuthState()
	state.Set("access", "refresh", time.Now().Add(time.Hour), "user@example.com", "admin")

	client := auth.NewClient(srv.URL)
	// Error is expected but state must still be cleared.
	_ = client.Logout(context.Background(), state)
	assert.Equal(t, "", state.Bearer(), "state must be cleared even when logout HTTP call fails")
}

// TestAuthClient_NeverWritesTokenToDisk verifies that Login against a fake server
// does not create new files under ~/.argus/ (only Phase 1's config.yaml may exist).
func TestAuthClient_NeverWritesTokenToDisk(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "test-access",
			"refresh_token": "test-refresh",
			"user": map[string]any{
				"email": "admin@argus.io",
				"role":  "admin",
			},
		})
	}))
	defer srv.Close()

	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)
	argusDir := filepath.Join(homeDir, ".argus")

	// Snapshot files before login.
	beforeFiles := listDir(t, argusDir)

	client := auth.NewClient(srv.URL)
	_, _, err = client.Login(context.Background(), "admin@argus.io", "password")
	require.NoError(t, err)

	// Snapshot files after login.
	afterFiles := listDir(t, argusDir)

	// Only the pre-existing config.yaml (from Phase 1) should be there; no token files.
	for f := range afterFiles {
		if f == "config.yaml" {
			continue
		}
		_, existed := beforeFiles[f]
		assert.True(t, existed, "unexpected new file %q created in ~/.argus/ during Login()", f)
	}
}

// listDir returns a set of filenames in dir. Returns empty set if dir doesn't exist.
func listDir(t *testing.T, dir string) map[string]struct{} {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return map[string]struct{}{}
	}
	require.NoError(t, err)
	out := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		out[e.Name()] = struct{}{}
	}
	return out
}
