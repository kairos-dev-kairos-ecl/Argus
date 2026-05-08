// Package auth provides JWT-in-memory auth state and client for the Argus TUI.
//
// Security invariants (locked from Phase 2 plan):
//   - All token fields are private. No MarshalJSON, no MarshalYAML, no YAML tags.
//   - Tokens are NEVER written to disk.
//   - Tokens are NEVER logged (no zap reference with token values).
//   - ClearOnLogout zeroes ALL fields; always called on logout and on refresh failure.
package auth

import (
	"sync"
	"time"
)

// AuthState holds JWT credentials in memory. It is mutex-protected for
// concurrent access from the HTTP client (reads) and refresh path (writes).
//
// SECURITY: This struct has no serialization methods. It must never be written
// to disk, logged, or transmitted. ClearOnLogout() must be called on app exit.
type AuthState struct {
	mu           sync.RWMutex
	accessToken  string
	refreshToken string
	expiresAt    time.Time
	email        string
	role         string
}

// NewAuthState returns a zero-valued AuthState ready for use.
func NewAuthState() *AuthState {
	return &AuthState{}
}

// Set populates all fields atomically under the write lock.
func (s *AuthState) Set(accessToken, refreshToken string, expiresAt time.Time, email, role string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accessToken = accessToken
	s.refreshToken = refreshToken
	s.expiresAt = expiresAt
	s.email = email
	s.role = role
}

// Bearer returns the current access token under the read lock.
func (s *AuthState) Bearer() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.accessToken
}

// RefreshToken returns the current refresh token under the read lock.
func (s *AuthState) RefreshToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.refreshToken
}

// ExpiresAt returns the token expiry time under the read lock.
func (s *AuthState) ExpiresAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.expiresAt
}

// Email returns the authenticated user's email under the read lock.
func (s *AuthState) Email() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.email
}

// Role returns the authenticated user's role under the read lock.
func (s *AuthState) Role() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.role
}

// IsAuthenticated returns true when a non-empty access token is present.
func (s *AuthState) IsAuthenticated() bool {
	return s.Bearer() != ""
}

// ClearOnLogout zeros every field under the write lock. This MUST be called on
// logout, app exit, and on any refresh failure to ensure credentials do not
// linger in memory after a session ends.
func (s *AuthState) ClearOnLogout() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accessToken = ""
	s.refreshToken = ""
	s.expiresAt = time.Time{}
	s.email = ""
	s.role = ""
}
