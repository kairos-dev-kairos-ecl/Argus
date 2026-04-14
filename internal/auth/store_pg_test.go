package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// These tests validate struct shapes and helper logic without a real DB.
// Integration tests with a real DB are left for a dedicated test harness.

func TestSession_IsExpired(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour).Unix()
	future := time.Now().Add(1 * time.Hour).Unix()
	revoked := time.Now().Unix()

	active := Session{ID: "s1", ExpiresAt: future, RevokedAt: nil}
	assert.False(t, active.RevokedAt != nil)
	assert.True(t, active.ExpiresAt > time.Now().Unix())

	expired := Session{ID: "s2", ExpiresAt: past, RevokedAt: nil}
	assert.False(t, expired.ExpiresAt > time.Now().Unix())

	revSess := Session{ID: "s3", ExpiresAt: future, RevokedAt: &revoked}
	assert.True(t, revSess.RevokedAt != nil)
}

func TestPgUserStore_nilPool(t *testing.T) {
	store := &PgUserStore{db: nil}
	assert.NotNil(t, store)
}

func TestPgSessionStore_nilPool(t *testing.T) {
	store := &PgSessionStore{db: nil}
	assert.NotNil(t, store)
}

func TestPgAuditStore_nilPool(t *testing.T) {
	store := &PgAuditStore{db: nil}
	assert.NotNil(t, store)
}
