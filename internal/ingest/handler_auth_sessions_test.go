package ingest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/argusxdr/argus/internal/auth"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// FakeSessionStore for testing
type FakeSessionStore struct {
	sessions map[string]*auth.Session
}

func NewFakeSessionStore() *FakeSessionStore {
	return &FakeSessionStore{
		sessions: make(map[string]*auth.Session),
	}
}

func (f *FakeSessionStore) GetSessionByUserID(ctx context.Context, userID string) ([]auth.Session, error) {
	var sessions []auth.Session
	for _, sess := range f.sessions {
		if sess.UserID == userID {
			sessions = append(sessions, *sess)
		}
	}
	return sessions, nil
}

func (f *FakeSessionStore) GetSessionByHash(ctx context.Context, hash string) (*auth.Session, error) {
	for _, sess := range f.sessions {
		if sess.RefreshTokenHash == hash {
			return sess, nil
		}
	}
	return nil, nil
}

func (f *FakeSessionStore) RevokeSession(ctx context.Context, sessionID string) error {
	if sess, ok := f.sessions[sessionID]; ok {
		now := time.Now().Unix()
		sess.RevokedAt = &now
	}
	return nil
}

func (f *FakeSessionStore) CheckTokenRevocation(ctx context.Context, tokenHash string) (bool, error) {
	return false, nil
}

func (f *FakeSessionStore) CreateSession(ctx context.Context, sess *auth.Session) error {
	f.sessions[sess.ID] = sess
	return nil
}

func (f *FakeSessionStore) UpdateSessionHash(ctx context.Context, sessionID, newHash string) error {
	if sess, ok := f.sessions[sessionID]; ok {
		sess.RefreshTokenHash = newHash
	}
	return nil
}

func (f *FakeSessionStore) RevokeTokenHash(ctx context.Context, tokenHash string, expiresAt time.Time) error {
	return nil
}

func TestSessionHandlers_ListSessions(t *testing.T) {
	// Set up test data
	testUserID := uuid.New()
	testSessionID := "session-1"
	fakeStore := NewFakeSessionStore()
	now := time.Now()

	// Current session
	fakeStore.sessions["session-1"] = &auth.Session{
		ID:         "session-1",
		UserID:     testUserID.String(),
		UserAgent:  "Mozilla/5.0",
		IPAddress:  "192.168.1.1",
		CreatedAt:  now.Unix(),
		LastUsedAt: now.Unix(),
		ExpiresAt:  now.Add(7 * 24 * time.Hour).Unix(),
	}
	// Other active session
	fakeStore.sessions["session-2"] = &auth.Session{
		ID:         "session-2",
		UserID:     testUserID.String(),
		UserAgent:  "Chrome/91",
		IPAddress:  "10.0.0.1",
		CreatedAt:  now.Add(-1 * time.Hour).Unix(),
		LastUsedAt: now.Add(-10 * time.Minute).Unix(),
		ExpiresAt:  now.Add(7 * 24 * time.Hour).Unix(),
	}

	queryHandler := &QueryHandler{
		log: zap.NewNop(),
	}
	authSvc := &AuthService{
		SessionStore: fakeStore,
	}
	queryHandler.SetAuthService(authSvc)

	// Create context with user claims using the helper
	claims := &auth.Claims{
		UserID:    testUserID,
		SessionID: testSessionID,
	}
	ctx := auth.ContextWithClaims(context.Background(), claims)

	req := httptest.NewRequest("GET", "/api/v1/auth/sessions", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	queryHandler.handleListSessions(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "status code")

	var sessions []SessionResponse
	err := json.NewDecoder(w.Body).Decode(&sessions)
	assert.NoError(t, err, "decode response")
	assert.Equal(t, 2, len(sessions), "session count")

	// Verify is_current flag
	foundCurrent := false
	for _, sess := range sessions {
		if sess.IsCurrent {
			assert.Equal(t, testSessionID, sess.ID)
			foundCurrent = true
		}
	}
	assert.True(t, foundCurrent, "should have one session marked as current")
}

func TestSessionHandlers_RevokeSession(t *testing.T) {
	testUserID := uuid.New()
	otherUserID := uuid.New()
	now := time.Now()

	tests := []struct {
		name       string
		sessionID  string
		setup      func(*FakeSessionStore)
		expectCode int
	}{
		{
			name:       "revoke owned session",
			sessionID:  "session-1",
			expectCode: http.StatusNoContent,
			setup: func(f *FakeSessionStore) {
				f.sessions["session-1"] = &auth.Session{
					ID:        "session-1",
					UserID:    testUserID.String(),
					CreatedAt: now.Unix(),
					ExpiresAt: now.Add(7 * 24 * time.Hour).Unix(),
				}
			},
		},
		{
			name:       "session not found",
			sessionID:  "nonexistent",
			expectCode: http.StatusNotFound,
			setup:      func(f *FakeSessionStore) {},
		},
		{
			name:       "unauthorized if not owner",
			sessionID:  "other-user-session",
			expectCode: http.StatusNotFound,
			setup: func(f *FakeSessionStore) {
				f.sessions["other-user-session"] = &auth.Session{
					ID:        "other-user-session",
					UserID:    otherUserID.String(),
					CreatedAt: now.Unix(),
					ExpiresAt: now.Add(7 * 24 * time.Hour).Unix(),
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeStore := NewFakeSessionStore()
			tt.setup(fakeStore)

			queryHandler := &QueryHandler{
				log: zap.NewNop(),
			}
			authSvc := &AuthService{
				SessionStore: fakeStore,
			}
			queryHandler.SetAuthService(authSvc)

			claims := &auth.Claims{
				UserID:    testUserID,
				SessionID: "current-session",
			}
			ctx := auth.ContextWithClaims(context.Background(), claims)

			req := httptest.NewRequest("DELETE", "/api/v1/auth/sessions/"+tt.sessionID, nil).WithContext(ctx)
			w := httptest.NewRecorder()

			queryHandler.handleRevokeSession(w, req)
			assert.Equal(t, tt.expectCode, w.Code)
		})
	}
}

func TestSessionHandlers_RevokeOtherSessions(t *testing.T) {
	userID := uuid.New()
	fakeStore := NewFakeSessionStore()

	now := time.Now()
	userIDStr := userID.String()

	// Create 3 sessions: current + 2 others
	fakeStore.sessions["session-current"] = &auth.Session{
		ID:        "session-current",
		UserID:    userIDStr,
		CreatedAt: now.Unix(),
		ExpiresAt: now.Add(7 * 24 * time.Hour).Unix(),
	}
	fakeStore.sessions["session-other-1"] = &auth.Session{
		ID:        "session-other-1",
		UserID:    userIDStr,
		CreatedAt: now.Add(-1 * time.Hour).Unix(),
		ExpiresAt: now.Add(7 * 24 * time.Hour).Unix(),
	}
	fakeStore.sessions["session-other-2"] = &auth.Session{
		ID:        "session-other-2",
		UserID:    userIDStr,
		CreatedAt: now.Add(-2 * time.Hour).Unix(),
		ExpiresAt: now.Add(7 * 24 * time.Hour).Unix(),
	}

	queryHandler := &QueryHandler{
		log: zap.NewNop(),
	}
	authSvc := &AuthService{
		SessionStore: fakeStore,
	}
	queryHandler.SetAuthService(authSvc)

	claims := &auth.Claims{
		UserID:    userID,
		SessionID: "session-current",
	}
	ctx := auth.ContextWithClaims(context.Background(), claims)

	req := httptest.NewRequest("DELETE", "/api/v1/auth/sessions", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	queryHandler.handleRevokeOtherSessions(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]int
	json.NewDecoder(w.Body).Decode(&response)
	assert.Equal(t, 2, response["revoked"])

	// Verify current session is still active (not revoked)
	current, _ := fakeStore.GetSessionByUserID(context.Background(), userIDStr)
	// Filter to only active sessions (not revoked)
	var activeSessions []auth.Session
	for _, sess := range current {
		if sess.RevokedAt == nil {
			activeSessions = append(activeSessions, sess)
		}
	}
	assert.Len(t, activeSessions, 1)
	assert.Equal(t, "session-current", activeSessions[0].ID)
	assert.Nil(t, activeSessions[0].RevokedAt)
}
