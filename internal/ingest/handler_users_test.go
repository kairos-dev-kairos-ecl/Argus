package ingest

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestHandleListUsers_NoAuthService_503(t *testing.T) {
	h := NewQueryHandler(nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	w := httptest.NewRecorder()
	h.handleListUsers(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleCreateUser_NoAuthService_503(t *testing.T) {
	h := NewQueryHandler(nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", nil)
	w := httptest.NewRecorder()
	h.handleCreateUser(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
