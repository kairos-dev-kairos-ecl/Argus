package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mr-tron/base58"
)

const APIKeyPrefix = "argus_sk_"

var (
	ErrAPIKeyNotFound = errors.New("api key not found")
	ErrAPIKeyRevoked  = errors.New("api key revoked")
	ErrAPIKeyExpired  = errors.New("api key expired")
)

type APIKey struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	AppID      string // text NOT NULL in DB; defaults to "system" when not specified
	Name       string
	KeyPrefix  string
	KeyHash    string
	Scopes     []string
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
}

type ApiKeyStore interface {
	Create(ctx context.Context, k *APIKey) error
	GetByHash(ctx context.Context, hash string) (*APIKey, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*APIKey, error)
	GetByID(ctx context.Context, id uuid.UUID) (*APIKey, error)
	Revoke(ctx context.Context, id uuid.UUID) error
	TouchLastUsed(ctx context.Context, id uuid.UUID, t time.Time) error
}

// GenerateAPIKey returns (fullKey, prefix12, hashHex).
// fullKey = "argus_sk_" + base58(32 random bytes). Shown to user once.
// prefix12 = first 12 chars of fullKey (for UI display).
// hashHex = sha256(fullKey) as lowercase hex.
func GenerateAPIKey() (string, string, string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", "", fmt.Errorf("rand: %w", err)
	}
	full := APIKeyPrefix + base58.Encode(buf)
	prefix := full
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	return full, prefix, HashAPIKey(full), nil
}

// HashAPIKey returns sha256 hex (lowercase, 64 chars)
func HashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// ValidateAPIKey is a package-level function, NOT a method on ApiKeyStore.
// It hashes the raw key internally (callers pass the raw key from the header),
// looks it up, and returns error if not found, revoked, or expired.
// Updates last_used_at on success (best-effort, non-blocking).
func ValidateAPIKey(ctx context.Context, store ApiKeyStore, rawKey string) (*APIKey, error) {
	k, err := store.GetByHash(ctx, HashAPIKey(rawKey))
	if err != nil || k == nil {
		return nil, ErrAPIKeyNotFound
	}
	if k.RevokedAt != nil {
		return nil, ErrAPIKeyRevoked
	}
	if k.ExpiresAt != nil && k.ExpiresAt.Before(time.Now()) {
		return nil, ErrAPIKeyExpired
	}
	// Best-effort touch; do not fail validation if it errors.
	_ = store.TouchLastUsed(ctx, k.ID, time.Now())
	return k, nil
}
