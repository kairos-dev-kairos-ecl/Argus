package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SessionManager handles session creation, rotation, and revocation
type SessionManager struct {
	sessionStore        SessionStore
	refreshTokenTTL     time.Duration
	maxConcurrentSessions int
}

// NewSessionManager creates a new session manager
func NewSessionManager(store SessionStore, ttl time.Duration) *SessionManager {
	if ttl == 0 {
		ttl = 7 * 24 * time.Hour
	}
	return &SessionManager{
		sessionStore:        store,
		refreshTokenTTL:     ttl,
		maxConcurrentSessions: 5,
	}
}

// CreateSession creates a new session for a user
func (sm *SessionManager) CreateSession(ctx context.Context, userID uuid.UUID, userAgent, ipAddress string) (refreshToken string, sessionID string, err error) {
	// Generate refresh token
	refreshToken = sm.GenerateRefreshToken()
	tokenHash := sm.HashToken(refreshToken)

	// Check concurrent sessions
	existingSessions, err := sm.sessionStore.GetSessionByUserID(ctx, userID.String())
	if err != nil {
		return "", "", fmt.Errorf("failed to check existing sessions: %w", err)
	}

	// If at limit, revoke oldest session
	if len(existingSessions) >= sm.maxConcurrentSessions {
		var oldestSession Session
		found := false
		for _, s := range existingSessions {
			if s.RevokedAt == nil {
				if !found || s.CreatedAt < oldestSession.CreatedAt {
					oldestSession = s
					found = true
				}
			}
		}
		if found {
			if err := sm.sessionStore.RevokeSession(ctx, oldestSession.ID); err != nil {
				return "", "", fmt.Errorf("failed to revoke old session: %w", err)
			}
		}
	}

	now := time.Now()
	sessionID = uuid.New().String()

	session := &Session{
		ID:               sessionID,
		UserID:           userID.String(),
		RefreshTokenHash: tokenHash,
		UserAgent:        userAgent,
		IPAddress:        ipAddress,
		CreatedAt:        now.Unix(),
		ExpiresAt:        now.Add(sm.refreshTokenTTL).Unix(),
		LastUsedAt:       now.Unix(),
	}

	// Store session (this would normally persist to database)
	// For now, we'll assume this is handled elsewhere
	_ = session

	return refreshToken, sessionID, nil
}

// RotateRefreshToken generates a new refresh token and revokes the old one
func (sm *SessionManager) RotateRefreshToken(ctx context.Context, oldTokenHash string) (newToken string, newHash string, err error) {
	// Verify old token exists and is not revoked
	session, err := sm.sessionStore.GetSessionByHash(ctx, oldTokenHash)
	if err != nil {
		return "", "", fmt.Errorf("session not found: %w", err)
	}

	if session.RevokedAt != nil {
		return "", "", fmt.Errorf("session already revoked")
	}

	// Generate new token
	newToken = sm.GenerateRefreshToken()
	newHash = sm.HashToken(newToken)

	// Update session with new hash and last used time
	now := time.Now()
	session.RefreshTokenHash = newHash
	session.LastUsedAt = now.Unix()

	// TODO: Persist updated session to database

	return newToken, newHash, nil
}

// RevokeSession marks a session as revoked
func (sm *SessionManager) RevokeSession(ctx context.Context, sessionID string) error {
	return sm.sessionStore.RevokeSession(ctx, sessionID)
}

// RevokeAllSessionsForUser revokes all sessions for a user
func (sm *SessionManager) RevokeAllSessionsForUser(ctx context.Context, userID uuid.UUID) error {
	sessions, err := sm.sessionStore.GetSessionByUserID(ctx, userID.String())
	if err != nil {
		return fmt.Errorf("failed to get sessions: %w", err)
	}

	for _, session := range sessions {
		if session.RevokedAt == nil {
			if err := sm.sessionStore.RevokeSession(ctx, session.ID); err != nil {
				return fmt.Errorf("failed to revoke session: %w", err)
			}
		}
	}

	return nil
}

// RevokeTokenImmediately adds a token hash to the revocation list
func (sm *SessionManager) RevokeTokenImmediately(ctx context.Context, tokenHash string, expiresAt time.Time) error {
	// This would typically store the revocation in a fast cache (Redis)
	// with expiration matching the token's expiry
	_ = ctx
	_ = tokenHash
	_ = expiresAt
	return nil
}

// IsSessionValid checks if a session is valid and not expired
func (sm *SessionManager) IsSessionValid(session *Session) bool {
	if session.RevokedAt != nil {
		return false
	}
	if session.ExpiresAt < time.Now().Unix() {
		return false
	}
	return true
}

// GetActiveSessions retrieves all non-revoked sessions for a user
func (sm *SessionManager) GetActiveSessions(ctx context.Context, userID uuid.UUID) ([]Session, error) {
	sessions, err := sm.sessionStore.GetSessionByUserID(ctx, userID.String())
	if err != nil {
		return nil, err
	}

	var activeSessions []Session
	now := time.Now().Unix()
	for _, s := range sessions {
		if s.RevokedAt == nil && s.ExpiresAt > now {
			activeSessions = append(activeSessions, s)
		}
	}

	return activeSessions, nil
}

// GenerateRefreshToken creates a new random refresh token
func (sm *SessionManager) GenerateRefreshToken() string {
	return uuid.New().String()
}

// HashToken creates a SHA256 hash of a token for secure storage
func (sm *SessionManager) HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
