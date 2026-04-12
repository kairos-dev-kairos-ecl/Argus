package ingest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"valid bearer token", "Bearer test-token-123", "test-token-123"},
		{"missing bearer prefix", "Basic dGVzdDp0ZXN0", ""},
		{"empty header", "", ""},
		{"malformed header", "Bearer", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractBearerToken(tt.header)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAuthValidator_ValidateKey_TestMode(t *testing.T) {
	// Create test validator with test mode enabled
	testAppIDs := map[string]string{
		"test-key-1": "app-1",
		"test-key-2": "app-2",
	}
	av := NewTestAuthValidator(zap.NewNop(), testAppIDs)
	assert.NotNil(t, av)

	// Test with valid key
	appID, err := av.ValidateKey(context.Background(), "test-key-1")
	assert.NoError(t, err)
	assert.Equal(t, "app-1", appID)

	// Test with invalid key
	_, err2 := av.ValidateKey(context.Background(), "invalid-key")
	assert.Error(t, err2)

	// Test with empty key
	_, err3 := av.ValidateKey(context.Background(), "")
	assert.Error(t, err3)
	assert.Equal(t, "api key cannot be empty", err3.Error())
}

func TestAuthValidator_HTTPMiddleware(t *testing.T) {
	av := NewTestAuthValidator(zap.NewNop(), map[string]string{"valid-key": "app-1"})

	// Test 1: Missing authorization header
	middleware := av.HTTPMiddleware()
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, `{"error":"unauthenticated"}`, rec.Body.String())

	// Test 2: Malformed authorization header
	handler2 := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("Authorization", "Basic invalid")
	rec2 := httptest.NewRecorder()
	handler2.ServeHTTP(rec2, req2)

	assert.Equal(t, http.StatusUnauthorized, rec2.Code)
}

func TestAppIDFromContext(t *testing.T) {
	// Test with appID in context
	ctx := context.WithValue(context.Background(), appIDKey, "test-app-id")
	appID, ok := AppIDFromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, "test-app-id", appID)

	// Test without appID in context
	ctx2 := context.Background()
	_, ok2 := AppIDFromContext(ctx2)
	assert.False(t, ok2)
}

func TestWrappedServerStream_Context(t *testing.T) {
	// Create a mock ServerStream
	mockStream := &mockServerStream{
		ctx: context.Background(),
	}

	// Create a wrapped stream with modified context
	newCtx := context.WithValue(context.Background(), appIDKey, "test-app")
	wrapped := &wrappedServerStream{
		ServerStream: mockStream,
		ctx:          newCtx,
	}

	// Verify the context is the wrapped one
	assert.Equal(t, newCtx, wrapped.Context())
	assert.NotEqual(t, mockStream.Context(), wrapped.Context())
}

func TestGRPCStreamInterceptor(t *testing.T) {
	av := NewTestAuthValidator(zap.NewNop(), map[string]string{"valid-key": "app-1"})
	interceptor := av.GRPCStreamInterceptor()

	// Test 1: Missing metadata
	mockStream := &mockServerStream{
		ctx: context.Background(),
	}

	err := interceptor(nil, mockStream, &grpc.StreamServerInfo{}, func(srv interface{}, ss grpc.ServerStream) error {
		return nil
	})

	assert.Error(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())

	// Test 2: Missing authorization header
	mockStream2 := &mockServerStream{
		ctx: metadata.NewIncomingContext(context.Background(), metadata.MD{}),
	}

	err2 := interceptor(nil, mockStream2, &grpc.StreamServerInfo{}, func(srv interface{}, ss grpc.ServerStream) error {
		return nil
	})

	assert.Error(t, err2)
	st2, ok2 := status.FromError(err2)
	assert.True(t, ok2)
	assert.Equal(t, codes.Unauthenticated, st2.Code())

	// Test 3: Invalid authorization header format
	md := metadata.Pairs("authorization", "Basic invalid")
	mockStream3 := &mockServerStream{
		ctx: metadata.NewIncomingContext(context.Background(), md),
	}

	err3 := interceptor(nil, mockStream3, &grpc.StreamServerInfo{}, func(srv interface{}, ss grpc.ServerStream) error {
		return nil
	})

	assert.Error(t, err3)
	st3, ok3 := status.FromError(err3)
	assert.True(t, ok3)
	assert.Equal(t, codes.Unauthenticated, st3.Code())
}

// Mock ServerStream for testing
type mockServerStream struct {
	ctx context.Context
}

func (m *mockServerStream) SetHeader(metadata.MD) error {
	return nil
}

func (m *mockServerStream) SendHeader(metadata.MD) error {
	return nil
}

func (m *mockServerStream) SetTrailer(metadata.MD) {
}

func (m *mockServerStream) Context() context.Context {
	return m.ctx
}

func (m *mockServerStream) SendMsg(interface{}) error {
	return nil
}

func (m *mockServerStream) RecvMsg(interface{}) error {
	return nil
}

// Test bcrypt key hashing integration
func TestAuthValidator_BcryptIntegration(t *testing.T) {
	// This test demonstrates the bcrypt pattern used in ValidateKey
	rawKey := "test-api-key-12345"

	// Hash the key
	hash, err := bcrypt.GenerateFromPassword([]byte(rawKey), bcrypt.DefaultCost)
	require.NoError(t, err)

	// Verify the hash matches
	err = bcrypt.CompareHashAndPassword(hash, []byte(rawKey))
	assert.NoError(t, err)

	// Verify wrong key doesn't match
	err2 := bcrypt.CompareHashAndPassword(hash, []byte("wrong-key"))
	assert.Error(t, err2)
}

// Test cache TTL behavior
func TestAuthValidator_CacheTTL(t *testing.T) {
	av := NewTestAuthValidator(zap.NewNop(), map[string]string{"test-key": "app-1"})

	// Manually populate cache with expired entry
	expiredTime := time.Now().Add(-1 * time.Second)
	av.cache.Store("expired-key", cachedKey{
		appID:     "app-1",
		expiresAt: expiredTime,
	})

	// Attempt to validate should fail (expired in cache)
	_, err := av.ValidateKey(context.Background(), "expired-key")
	assert.Error(t, err)
	assert.Equal(t, "api key expired", err.Error())

	// Cache entry should be deleted
	_, ok := av.cache.Load("expired-key")
	assert.False(t, ok)
}

// Test revoked key caching
func TestAuthValidator_RevokedKey(t *testing.T) {
	av := NewTestAuthValidator(zap.NewNop(), map[string]string{"revoked-key": "app-1"})

	// Manually populate cache with revoked entry
	revokedTime := time.Now().Add(-1 * time.Hour)
	av.cache.Store("revoked-key", cachedKey{
		appID:     "app-1",
		expiresAt: time.Now().Add(1 * time.Hour),
		revokedAt: &revokedTime,
	})

	// Attempt to validate should fail (revoked)
	_, err := av.ValidateKey(context.Background(), "revoked-key")
	assert.Error(t, err)
	assert.Equal(t, "api key revoked", err.Error())

	// Cache entry should be deleted
	_, ok := av.cache.Load("revoked-key")
	assert.False(t, ok)
}
