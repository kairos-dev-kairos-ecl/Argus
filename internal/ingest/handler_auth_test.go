package ingest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestHandleSetup_NoAuthService_503(t *testing.T) {
	h := NewQueryHandler(nil, nil, zap.NewNop())
	// authService not set → 503
	body := `{"email":"admin@example.com","password":"supersecret1234","display_name":"Admin","instance_name":"test","app_name":"myapp"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleSetup(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleLogin_NoAuthService_503(t *testing.T) {
	h := NewQueryHandler(nil, nil, zap.NewNop())
	body := `{"email":"user@example.com","password":"pass"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleLogin(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleRefreshToken_NoAuthService_503(t *testing.T) {
	h := NewQueryHandler(nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	w := httptest.NewRecorder()
	h.handleRefreshToken(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleLogout_NoAuthService_200(t *testing.T) {
	h := NewQueryHandler(nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	w := httptest.NewRecorder()
	h.handleLogout(w, req)
	// logout with no session is always 200 (idempotent)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthServiceAvailable_nilCheck(t *testing.T) {
	h := NewQueryHandler(nil, nil, zap.NewNop())
	assert.False(t, h.authAvailable())

	h2 := NewQueryHandler(nil, nil, zap.NewNop())
	h2.SetAuthService(&AuthService{})
	assert.True(t, h2.authAvailable())
}

func TestLoginRequest_Decode(t *testing.T) {
	body := `{"email":"a@b.com","password":"pw"}`
	var req loginRequest
	err := json.NewDecoder(bytes.NewBufferString(body)).Decode(&req)
	require.NoError(t, err)
	assert.Equal(t, "a@b.com", req.Email)
	assert.Equal(t, "pw", req.Password)
}
