package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// fakeApiKeyStore implements ApiKeyStore for testing
type fakeApiKeyStore struct {
	keys map[string]*APIKey
}

func newFakeApiKeyStore() *fakeApiKeyStore {
	return &fakeApiKeyStore{
		keys: make(map[string]*APIKey),
	}
}

func (f *fakeApiKeyStore) Create(ctx context.Context, k *APIKey) error {
	f.keys[k.KeyHash] = k
	return nil
}

func (f *fakeApiKeyStore) GetByHash(ctx context.Context, hash string) (*APIKey, error) {
	key, ok := f.keys[hash]
	if !ok {
		return nil, ErrAPIKeyNotFound
	}
	return key, nil
}

func (f *fakeApiKeyStore) ListByUser(ctx context.Context, userID uuid.UUID) ([]*APIKey, error) {
	return nil, nil
}

func (f *fakeApiKeyStore) GetByID(ctx context.Context, id uuid.UUID) (*APIKey, error) {
	return nil, nil
}

func (f *fakeApiKeyStore) Revoke(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (f *fakeApiKeyStore) TouchLastUsed(ctx context.Context, id uuid.UUID, t time.Time) error {
	return nil
}

func TestAPIKeyMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		header         string
		store          *fakeApiKeyStore
		requiredScope  string
		expectedStatus int
		expectedBody   string
		setupStore     func(*fakeApiKeyStore)
	}{
		{
			name:           "missing X-Argus-API-Key header",
			header:         "",
			requiredScope:  "signals:write",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `"missing api key"`,
			setupStore:     func(f *fakeApiKeyStore) {},
		},
		{
			name:           "invalid/unknown api key",
			header:         "argus_sk_invalid",
			requiredScope:  "signals:write",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `"invalid api key"`,
			setupStore:     func(f *fakeApiKeyStore) {},
		},
		{
			name:           "revoked api key",
			header:         "test_key_revoked",
			requiredScope:  "signals:write",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `"invalid api key"`,
			setupStore: func(f *fakeApiKeyStore) {
				now := time.Now()
				k := &APIKey{
					ID:        uuid.New(),
					UserID:    uuid.New(),
					Name:      "revoked",
					KeyPrefix: "argus_sk_test",
					KeyHash:   HashAPIKey("test_key_revoked"),
					Scopes:    []string{"signals:write"},
					RevokedAt: &now,
					CreatedAt: now,
				}
				f.Create(context.Background(), k)
			},
		},
		{
			name:           "expired api key",
			header:         "test_key_expired",
			requiredScope:  "signals:write",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `"invalid api key"`,
			setupStore: func(f *fakeApiKeyStore) {
				now := time.Now()
				past := now.Add(-1 * time.Hour)
				k := &APIKey{
					ID:        uuid.New(),
					UserID:    uuid.New(),
					Name:      "expired",
					KeyPrefix: "argus_sk_test",
					KeyHash:   HashAPIKey("test_key_expired"),
					Scopes:    []string{"signals:write"},
					ExpiresAt: &past,
					CreatedAt: now,
				}
				f.Create(context.Background(), k)
			},
		},
		{
			name:           "valid key but missing required scope",
			header:         "test_key_noscope",
			requiredScope:  "signals:write",
			expectedStatus: http.StatusForbidden,
			expectedBody:   `"insufficient scope"`,
			setupStore: func(f *fakeApiKeyStore) {
				now := time.Now()
				k := &APIKey{
					ID:        uuid.New(),
					UserID:    uuid.New(),
					Name:      "noscope",
					KeyPrefix: "argus_sk_test",
					KeyHash:   HashAPIKey("test_key_noscope"),
					Scopes:    []string{"signals:read"},
					CreatedAt: now,
				}
				f.Create(context.Background(), k)
			},
		},
		{
			name:           "valid key with required scope",
			header:         "test_key_valid",
			requiredScope:  "signals:write",
			expectedStatus: http.StatusOK,
			expectedBody:   "ok",
			setupStore: func(f *fakeApiKeyStore) {
				now := time.Now()
				k := &APIKey{
					ID:        uuid.New(),
					UserID:    uuid.New(),
					Name:      "valid",
					KeyPrefix: "argus_sk_test",
					KeyHash:   HashAPIKey("test_key_valid"),
					Scopes:    []string{"signals:write", "signals:read"},
					CreatedAt: now,
				}
				f.Create(context.Background(), k)
			},
		},
		{
			name:           "valid key no required scope check",
			header:         "test_key_valid2",
			requiredScope:  "",
			expectedStatus: http.StatusOK,
			expectedBody:   "ok",
			setupStore: func(f *fakeApiKeyStore) {
				now := time.Now()
				k := &APIKey{
					ID:        uuid.New(),
					UserID:    uuid.New(),
					Name:      "valid2",
					KeyPrefix: "argus_sk_test",
					KeyHash:   HashAPIKey("test_key_valid2"),
					Scopes:    []string{"signals:read"},
					CreatedAt: now,
				}
				f.Create(context.Background(), k)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeApiKeyStore()
			tt.setupStore(store)

			// Create a simple handler that returns 200 with "ok"
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
			})

			// Wrap with middleware
			middleware := APIKeyMiddleware(store, tt.requiredScope)
			handler := middleware(nextHandler)

			// Create request
			req := httptest.NewRequest("POST", "/v1/signals", nil)
			if tt.header != "" {
				req.Header.Set("X-Argus-API-Key", tt.header)
			}

			// Record response
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			// Assert status
			assert.Equal(t, tt.expectedStatus, w.Code, "status code mismatch for test: %s", tt.name)

			// Assert body contains expected string
			assert.Contains(t, w.Body.String(), tt.expectedBody, "response body mismatch for test: %s", tt.name)
		})
	}
}

func TestAPIKeyFromContext(t *testing.T) {
	t.Run("api key present in context", func(t *testing.T) {
		key := &APIKey{
			ID:        uuid.New(),
			UserID:    uuid.New(),
			Name:      "test",
			KeyPrefix: "argus_sk_test",
			Scopes:    []string{"signals:write"},
		}
		ctx := context.WithValue(context.Background(), APIKeyContextKey, key)

		retrieved, ok := APIKeyFromContext(ctx)
		assert.True(t, ok, "should find API key in context")
		assert.Equal(t, key, retrieved, "should return the same API key")
	})

	t.Run("api key not in context", func(t *testing.T) {
		ctx := context.Background()

		retrieved, ok := APIKeyFromContext(ctx)
		assert.False(t, ok, "should not find API key in context")
		assert.Nil(t, retrieved, "should return nil when key not in context")
	})
}
