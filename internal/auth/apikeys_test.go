package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockApiKeyStore is an in-memory implementation for unit tests
type mockApiKeyStore struct {
	keys map[string]*APIKey // hash -> key
	byID map[uuid.UUID]*APIKey
}

func newMockApiKeyStore() *mockApiKeyStore {
	return &mockApiKeyStore{
		keys: make(map[string]*APIKey),
		byID: make(map[uuid.UUID]*APIKey),
	}
}

func (m *mockApiKeyStore) Create(ctx context.Context, k *APIKey) error {
	m.keys[k.KeyHash] = k
	m.byID[k.ID] = k
	return nil
}

func (m *mockApiKeyStore) GetByHash(ctx context.Context, hash string) (*APIKey, error) {
	k, ok := m.keys[hash]
	if !ok {
		return nil, nil
	}
	return k, nil
}

func (m *mockApiKeyStore) GetByID(ctx context.Context, id uuid.UUID) (*APIKey, error) {
	k, ok := m.byID[id]
	if !ok {
		return nil, nil
	}
	return k, nil
}

func (m *mockApiKeyStore) ListByUser(ctx context.Context, userID uuid.UUID) ([]*APIKey, error) {
	var result []*APIKey
	for _, k := range m.byID {
		if k.UserID == userID {
			result = append(result, k)
		}
	}
	return result, nil
}

func (m *mockApiKeyStore) Revoke(ctx context.Context, id uuid.UUID) error {
	k, ok := m.byID[id]
	if ok {
		now := time.Now()
		k.RevokedAt = &now
	}
	return nil
}

func (m *mockApiKeyStore) TouchLastUsed(ctx context.Context, id uuid.UUID, t time.Time) error {
	k, ok := m.byID[id]
	if ok {
		k.LastUsedAt = &t
	}
	return nil
}

// Tests for GenerateAPIKey
func TestGenerateAPIKey(t *testing.T) {
	t.Run("returns key with prefix", func(t *testing.T) {
		fullKey, prefix, _, err := GenerateAPIKey()
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(fullKey, APIKeyPrefix), "full key should start with %s", APIKeyPrefix)
		assert.Equal(t, APIKeyPrefix, prefix[:len(APIKeyPrefix)], "prefix should start with argus_sk_")
		assert.Equal(t, 12, len(prefix), "prefix should be 12 chars")
		assert.True(t, len(fullKey) > 12, "full key should be longer than prefix")
	})

	t.Run("generates high entropy keys", func(t *testing.T) {
		key1, _, _, err := GenerateAPIKey()
		require.NoError(t, err)
		key2, _, _, err := GenerateAPIKey()
		require.NoError(t, err)
		assert.NotEqual(t, key1, key2, "two generated keys should be different")
	})

	t.Run("hash is deterministic 64-char hex", func(t *testing.T) {
		fullKey, _, hash1, err := GenerateAPIKey()
		require.NoError(t, err)

		// Re-hash the same key
		hash2 := HashAPIKey(fullKey)
		assert.Equal(t, hash1, hash2, "hash should be deterministic")
		assert.Equal(t, 64, len(hash1), "hash should be 64 hex chars")
		// Verify it's lowercase hex
		assert.Equal(t, hash1, strings.ToLower(hash1), "hash should be lowercase")
	})
}

// Tests for HashAPIKey
func TestHashAPIKey(t *testing.T) {
	t.Run("produces 64-char lowercase hex", func(t *testing.T) {
		hash := HashAPIKey("test_key_123")
		assert.Equal(t, 64, len(hash), "hash should be 64 chars")
		assert.Equal(t, hash, strings.ToLower(hash), "hash should be lowercase")
		// Verify it's valid hex (no non-hex chars)
		for _, c := range hash {
			assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'), "hash should be valid hex")
		}
	})

	t.Run("is deterministic", func(t *testing.T) {
		key := "argus_sk_test123"
		hash1 := HashAPIKey(key)
		hash2 := HashAPIKey(key)
		assert.Equal(t, hash1, hash2)
	})

	t.Run("changes with input", func(t *testing.T) {
		hash1 := HashAPIKey("key1")
		hash2 := HashAPIKey("key2")
		assert.NotEqual(t, hash1, hash2)
	})
}

// Tests for ValidateAPIKey
func TestValidateAPIKey(t *testing.T) {
	ctx := context.Background()
	store := newMockApiKeyStore()

	t.Run("valid key returns key and updates last_used_at", func(t *testing.T) {
		userID := uuid.New()
		fullKey, prefix, hash := "argus_sk_testkey", "argus_sk_", HashAPIKey("argus_sk_testkey")

		// Create and store key
		now := time.Now()
		apiKey := &APIKey{
			ID:        uuid.New(),
			UserID:    userID,
			Name:      "test",
			KeyPrefix: prefix,
			KeyHash:   hash,
			Scopes:    []string{"signals:write"},
			CreatedAt: now,
		}
		err := store.Create(ctx, apiKey)
		require.NoError(t, err)

		// Validate
		validated, err := ValidateAPIKey(ctx, store, fullKey)
		require.NoError(t, err)
		assert.NotNil(t, validated)
		assert.Equal(t, userID, validated.UserID)

		// Check that last_used_at was updated
		storedKey, _ := store.GetByID(ctx, apiKey.ID)
		assert.NotNil(t, storedKey.LastUsedAt, "last_used_at should be updated")
	})

	t.Run("unknown hash returns ErrAPIKeyNotFound", func(t *testing.T) {
		validated, err := ValidateAPIKey(ctx, store, "unknown_key")
		assert.Equal(t, ErrAPIKeyNotFound, err)
		assert.Nil(t, validated)
	})

	t.Run("revoked key returns ErrAPIKeyRevoked", func(t *testing.T) {
		userID := uuid.New()
		fullKey := "argus_sk_revoked"
		hash := HashAPIKey(fullKey)

		now := time.Now()
		apiKey := &APIKey{
			ID:        uuid.New(),
			UserID:    userID,
			Name:      "revoked",
			KeyPrefix: "argus_sk_",
			KeyHash:   hash,
			CreatedAt: now,
			RevokedAt: &now,
		}
		err := store.Create(ctx, apiKey)
		require.NoError(t, err)

		validated, err := ValidateAPIKey(ctx, store, fullKey)
		assert.Equal(t, ErrAPIKeyRevoked, err)
		assert.Nil(t, validated)
	})

	t.Run("expired key returns ErrAPIKeyExpired", func(t *testing.T) {
		userID := uuid.New()
		fullKey := "argus_sk_expired"
		hash := HashAPIKey(fullKey)

		now := time.Now()
		expiredTime := now.Add(-1 * time.Hour) // 1 hour ago
		apiKey := &APIKey{
			ID:        uuid.New(),
			UserID:    userID,
			Name:      "expired",
			KeyPrefix: "argus_sk_",
			KeyHash:   hash,
			CreatedAt: now,
			ExpiresAt: &expiredTime,
		}
		err := store.Create(ctx, apiKey)
		require.NoError(t, err)

		validated, err := ValidateAPIKey(ctx, store, fullKey)
		assert.Equal(t, ErrAPIKeyExpired, err)
		assert.Nil(t, validated)
	})

	t.Run("future-expiring key validates", func(t *testing.T) {
		userID := uuid.New()
		fullKey := "argus_sk_future"
		hash := HashAPIKey(fullKey)

		now := time.Now()
		futureTime := now.Add(24 * time.Hour) // 24 hours from now
		apiKey := &APIKey{
			ID:        uuid.New(),
			UserID:    userID,
			Name:      "future",
			KeyPrefix: "argus_sk_",
			KeyHash:   hash,
			CreatedAt: now,
			ExpiresAt: &futureTime,
		}
		err := store.Create(ctx, apiKey)
		require.NoError(t, err)

		validated, err := ValidateAPIKey(ctx, store, fullKey)
		require.NoError(t, err)
		assert.NotNil(t, validated)
		assert.Equal(t, userID, validated.UserID)
	})

	t.Run("table-driven test suite", func(t *testing.T) {
		tests := []struct {
			name          string
			apiKey        *APIKey
			rawKey        string
			expectedErr   error
			shouldUpdate  bool
		}{
			{
				name: "valid active key",
				apiKey: &APIKey{
					ID:        uuid.New(),
					UserID:    uuid.New(),
					Name:      "valid",
					KeyPrefix: "argus_sk_",
					KeyHash:   HashAPIKey("argus_sk_valid"),
					CreatedAt: time.Now(),
				},
				rawKey:       "argus_sk_valid",
				expectedErr:  nil,
				shouldUpdate: true,
			},
			{
				name: "revoked key",
				apiKey: &APIKey{
					ID:        uuid.New(),
					UserID:    uuid.New(),
					Name:      "revoked",
					KeyPrefix: "argus_sk_",
					KeyHash:   HashAPIKey("argus_sk_revoked"),
					CreatedAt: time.Now(),
					RevokedAt: ptrTime(time.Now()),
				},
				rawKey:       "argus_sk_revoked",
				expectedErr:  ErrAPIKeyRevoked,
				shouldUpdate: false,
			},
			{
				name:          "unknown key",
				apiKey:        nil,
				rawKey:        "argus_sk_unknown",
				expectedErr:   ErrAPIKeyNotFound,
				shouldUpdate:  false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				testStore := newMockApiKeyStore()
				if tt.apiKey != nil {
					err := testStore.Create(ctx, tt.apiKey)
					require.NoError(t, err)
				}

				validated, err := ValidateAPIKey(ctx, testStore, tt.rawKey)

				if tt.expectedErr != nil {
					assert.Equal(t, tt.expectedErr, err)
					assert.Nil(t, validated)
				} else {
					assert.NoError(t, err)
					assert.NotNil(t, validated)

					if tt.shouldUpdate {
						stored, _ := testStore.GetByID(ctx, tt.apiKey.ID)
						assert.NotNil(t, stored.LastUsedAt)
					}
				}
			})
		}
	})
}

// Helper function to create a pointer to time.Time
func ptrTime(t time.Time) *time.Time {
	return &t
}
