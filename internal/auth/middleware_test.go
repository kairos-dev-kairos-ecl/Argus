package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestMain sets up shared test fixtures
func TestMain(m *testing.M) {
	// Just run tests - fixtures are per-test
	_ = m
}

// createTestTokenManager creates a token manager for testing
func createTestTokenManager(t *testing.T) *TokenManager {
	// Generate RSA key pair for testing
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "failed to generate RSA key")

	cfg := TokenConfig{
		PrivateKey:      privateKey,
		PublicKey:       &privateKey.PublicKey,
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
		Issuer:          "argus-xdr",
		Audience:        []string{"argus-xdr"},
	}
	return NewTokenManager(cfg)
}

// FakeSessionStore implements SessionStore for testing
type FakeSessionStore struct {
	sessions     map[string]*Session
	revocations  map[string]bool
}

func newFakeSessionStore() *FakeSessionStore {
	return &FakeSessionStore{
		sessions:    make(map[string]*Session),
		revocations: make(map[string]bool),
	}
}

func (s *FakeSessionStore) GetSessionByUserID(ctx context.Context, userID string) ([]Session, error) {
	var sessions []Session
	for _, session := range s.sessions {
		if session.UserID == userID {
			sessions = append(sessions, *session)
		}
	}
	return sessions, nil
}

func (s *FakeSessionStore) GetSessionByHash(ctx context.Context, hash string) (*Session, error) {
	if session, ok := s.sessions[hash]; ok {
		return session, nil
	}
	return nil, fmt.Errorf("session not found")
}

func (s *FakeSessionStore) RevokeSession(ctx context.Context, sessionID string) error {
	if _, ok := s.sessions[sessionID]; ok {
		delete(s.sessions, sessionID)
	}
	return nil
}

func (s *FakeSessionStore) CheckTokenRevocation(ctx context.Context, tokenHash string) (bool, error) {
	return s.revocations[tokenHash], nil
}

func (s *FakeSessionStore) CreateSession(ctx context.Context, sess *Session) error {
	s.sessions[sess.ID] = sess
	return nil
}

func (s *FakeSessionStore) UpdateSessionHash(ctx context.Context, sessionID, newHash string) error {
	if sess, ok := s.sessions[sessionID]; ok {
		sess.RefreshTokenHash = newHash
	}
	return nil
}

func (s *FakeSessionStore) RevokeTokenHash(ctx context.Context, tokenHash string, expiresAt time.Time) error {
	s.revocations[tokenHash] = true
	return nil
}

// TestRequireAuthWithValidJWT tests successful authentication with a valid JWT
func TestRequireAuthWithValidJWT(t *testing.T) {
	tm := createTestTokenManager(t)
	ss := newFakeSessionStore()
	al := &AuditLogger{} // No-op for testing
	log := zap.NewNop()

	// Create a valid JWT
	userID := uuid.New()
	token, err := tm.IssueAccessToken(userID, "test@example.com", "Test User", RoleAdmin, []string{PermUsersCreate})
	require.NoError(t, err, "failed to issue token")

	// Create RequireAuth middleware
	middleware := RequireAuth(tm, ss, al, log)
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		// Verify claims are in context
		claims, ok := ClaimsFromContext(r.Context())
		assert.True(t, ok, "claims should be in context")
		assert.NotNil(t, claims, "claims should not be nil")
		assert.Equal(t, userID, claims.UserID, "user ID should match")
		w.WriteHeader(http.StatusOK)
	})

	// Create request with valid token
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	// Call middleware
	handler := middleware(next)
	handler.ServeHTTP(w, req)

	assert.True(t, nextCalled, "next handler should be called")
	assert.Equal(t, http.StatusOK, w.Code, "should return 200 OK")
}

// TestRequireAuthMissingAuthHeader tests rejection without Authorization header
func TestRequireAuthMissingAuthHeader(t *testing.T) {
	tm := createTestTokenManager(t)
	ss := newFakeSessionStore()
	al := &AuditLogger{}
	log := zap.NewNop()

	middleware := RequireAuth(tm, ss, al, log)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})

	// Create request without Authorization header
	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	handler := middleware(next)
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "should return 401 Unauthorized")
}

// TestRequireAuthTamperedJWT tests rejection of tampered JWT
func TestRequireAuthTamperedJWT(t *testing.T) {
	tm := createTestTokenManager(t)
	ss := newFakeSessionStore()
	al := &AuditLogger{}
	log := zap.NewNop()

	middleware := RequireAuth(tm, ss, al, log)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})

	// Create request with tampered token (just a random string)
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.invalid.tampered")
	w := httptest.NewRecorder()

	handler := middleware(next)
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "should return 401 Unauthorized")
}

// TestRequireAuthRevokedSession tests rejection of revoked session
func TestRequireAuthRevokedSession(t *testing.T) {
	tm := createTestTokenManager(t)
	ss := newFakeSessionStore()
	al := &AuditLogger{}
	log := zap.NewNop()

	// Create a valid JWT
	userID := uuid.New()
	token, err := tm.IssueAccessToken(userID, "test@example.com", "Test User", RoleAdmin, nil)
	require.NoError(t, err)

	// Mark the token as revoked in session store
	tokenHash := HashToken(token)
	ss.revocations[tokenHash] = true

	middleware := RequireAuth(tm, ss, al, log)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called for revoked token")
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler := middleware(next)
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "should return 401 for revoked token")
}

// TestClaimsFromContext tests retrieving claims from context
func TestClaimsFromContext(t *testing.T) {
	userID := uuid.New()
	claims := &Claims{
		UserID:      userID,
		Email:       "test@example.com",
		DisplayName: "Test User",
		Role:        RoleAdmin,
	}

	// Create context with claims
	ctx := context.WithValue(context.Background(), claimsKey, claims)

	// Retrieve claims
	retrieved, ok := ClaimsFromContext(ctx)
	assert.True(t, ok, "should find claims in context")
	assert.Equal(t, claims, retrieved, "claims should match")
}

// TestClaimsFromContextNil tests retrieving claims from context without claims
func TestClaimsFromContextNil(t *testing.T) {
	ctx := context.Background()

	retrieved, ok := ClaimsFromContext(ctx)
	assert.False(t, ok, "should not find claims in empty context")
	assert.Nil(t, retrieved, "claims should be nil")
}

// TestUserIDFromContext tests retrieving user ID from context
func TestUserIDFromContext(t *testing.T) {
	userID := uuid.New()
	claims := &Claims{
		UserID: userID,
		Email:  "test@example.com",
	}
	ctx := context.WithValue(context.Background(), claimsKey, claims)

	retrieved, ok := UserIDFromContext(ctx)
	assert.True(t, ok, "should find user ID in context")
	assert.Equal(t, userID, retrieved, "user ID should match")
}

// TestUserIDFromContextNil tests retrieving user ID when claims are absent
func TestUserIDFromContextNil(t *testing.T) {
	ctx := context.Background()

	retrieved, ok := UserIDFromContext(ctx)
	assert.False(t, ok, "should not find user ID in empty context")
	assert.Equal(t, uuid.Nil, retrieved, "should return nil UUID")
}

// TestSessionIDFromContext tests retrieving session ID from context
func TestSessionIDFromContext(t *testing.T) {
	sessionID := "session-123"
	claims := &Claims{
		UserID:    uuid.New(),
		SessionID: sessionID,
	}
	ctx := context.WithValue(context.Background(), claimsKey, claims)

	retrieved, ok := SessionIDFromContext(ctx)
	assert.True(t, ok, "should find session ID in context")
	assert.Equal(t, sessionID, retrieved, "session ID should match")
}

// TestSessionIDFromContextNil tests retrieving session ID when claims are absent
func TestSessionIDFromContextNil(t *testing.T) {
	ctx := context.Background()

	retrieved, ok := SessionIDFromContext(ctx)
	assert.False(t, ok, "should not find session ID in empty context")
	assert.Empty(t, retrieved, "should return empty string")
}
