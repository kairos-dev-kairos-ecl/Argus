package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWTSigningAndVerification(t *testing.T) {
	// Generate RSA key pair for testing
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	config := TokenConfig{
		PrivateKey:      privateKey,
		PublicKey:       &privateKey.PublicKey,
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
		Issuer:          "test-issuer",
	}

	tm := NewTokenManager(config)

	userID := uuid.New()
	email := "test@example.com"
	displayName := "Test User"
	role := "admin"
	permissions := []string{"users:create", "rules:create"}

	// Test token issuance
	token, err := tm.IssueAccessToken(userID, email, displayName, role, permissions)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Test token verification
	claims, err := tm.VerifyAccessToken(token)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, email, claims.Email)
	assert.Equal(t, displayName, claims.DisplayName)
	assert.Equal(t, role, claims.Role)
	assert.Equal(t, permissions, claims.Permissions)

	// Verify not expired
	assert.False(t, tm.IsTokenExpired(claims))
}

func TestTokenExpiration(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	config := TokenConfig{
		PrivateKey:      privateKey,
		PublicKey:       &privateKey.PublicKey,
		AccessTokenTTL:  1 * time.Millisecond, // Very short TTL for testing
		RefreshTokenTTL: 7 * 24 * time.Hour,
	}

	tm := NewTokenManager(config)

	userID := uuid.New()
	token, err := tm.IssueAccessToken(userID, "test@example.com", "Test", "admin", []string{})
	require.NoError(t, err)

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	claims, err := tm.VerifyAccessToken(token)
	// Should fail due to expiration
	assert.Error(t, err)
	_ = claims
}

func TestRefreshTokenGeneration(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	config := TokenConfig{
		PrivateKey:      privateKey,
		PublicKey:       &privateKey.PublicKey,
	}

	tm := NewTokenManager(config)

	token1 := tm.IssueRefreshToken()
	token2 := tm.IssueRefreshToken()

	assert.NotEmpty(t, token1)
	assert.NotEmpty(t, token2)
	assert.NotEqual(t, token1, token2) // Tokens should be unique
}

func TestTokenTTLCalculation(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	config := TokenConfig{
		PrivateKey:      privateKey,
		PublicKey:       &privateKey.PublicKey,
		AccessTokenTTL:  15 * time.Minute,
	}

	tm := NewTokenManager(config)

	userID := uuid.New()
	token, err := tm.IssueAccessToken(userID, "test@example.com", "Test", "admin", []string{})
	require.NoError(t, err)

	claims, err := tm.VerifyAccessToken(token)
	require.NoError(t, err)

	ttl := tm.GetTokenTTL(claims)
	assert.Greater(t, ttl, 14*time.Minute) // Should be close to 15 minutes
	assert.Less(t, ttl, 15*time.Minute)
}

func TestInvalidTokenRejection(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	config := TokenConfig{
		PrivateKey:      privateKey,
		PublicKey:       &privateKey.PublicKey,
	}

	tm := NewTokenManager(config)

	// Try to verify invalid token
	_, err = tm.VerifyAccessToken("invalid.token.string")
	assert.Error(t, err)
}
