package ingest

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/argusxdr/argus/internal/auth"
	"go.uber.org/zap"
)

// SessionResponse represents a single session in the list response
type SessionResponse struct {
	ID         string    `json:"id"`
	UserAgent  string    `json:"user_agent"`
	IPAddress  string    `json:"ip_address"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at"`
	IsCurrent  bool      `json:"is_current"`
}

// handleListSessions returns all active (not revoked, not expired) sessions for the current user
func (h *QueryHandler) handleListSessions(w http.ResponseWriter, r *http.Request) {
	if !h.authAvailable() {
		jsonError(w, "auth service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Extract current userID and sessionID from context
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	currentSessionID, ok := auth.SessionIDFromContext(r.Context())
	if !ok {
		jsonError(w, "session not found in context", http.StatusInternalServerError)
		return
	}

	// List active sessions for user
	sessions, err := h.authService.SessionStore.GetSessionByUserID(r.Context(), userID.String())
	if err != nil {
		h.log.Error("failed to list sessions", zap.Error(err))
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Filter to active sessions and build response
	var activeSessions []SessionResponse
	now := time.Now()
	for _, sess := range sessions {
		// Skip revoked or expired sessions
		if sess.RevokedAt != nil || sess.ExpiresAt < now.Unix() {
			continue
		}

		activeSessions = append(activeSessions, SessionResponse{
			ID:         sess.ID,
			UserAgent:  sess.UserAgent,
			IPAddress:  sess.IPAddress,
			CreatedAt:  time.Unix(sess.CreatedAt, 0),
			LastUsedAt: time.Unix(sess.LastUsedAt, 0),
			IsCurrent:  sess.ID == currentSessionID,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(activeSessions)
}

// handleRevokeSession revokes a single session by ID
// Returns 404 if not found, 403 if not owned by current user, 204 on success
func (h *QueryHandler) handleRevokeSession(w http.ResponseWriter, r *http.Request) {
	if !h.authAvailable() {
		jsonError(w, "auth service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Extract current userID from context
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse session ID from URL path
	// Path format: /api/v1/auth/sessions/{id}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/auth/sessions/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		jsonError(w, "session id required", http.StatusBadRequest)
		return
	}
	sessionID := parts[0]

	// Fetch the session to verify ownership
	sessions, err := h.authService.SessionStore.GetSessionByUserID(r.Context(), userID.String())
	if err != nil {
		h.log.Error("failed to fetch sessions", zap.Error(err))
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Find the session and verify ownership
	var found bool
	for _, sess := range sessions {
		if sess.ID == sessionID {
			found = true
			break
		}
	}

	if !found {
		jsonError(w, "session not found", http.StatusNotFound)
		return
	}

	// Revoke the session
	if err := h.authService.SessionStore.RevokeSession(r.Context(), sessionID); err != nil {
		h.log.Error("failed to revoke session", zap.Error(err))
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleRevokeOtherSessions revokes all sessions for the current user EXCEPT the current one
func (h *QueryHandler) handleRevokeOtherSessions(w http.ResponseWriter, r *http.Request) {
	if !h.authAvailable() {
		jsonError(w, "auth service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Extract current userID and sessionID from context
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	currentSessionID, ok := auth.SessionIDFromContext(r.Context())
	if !ok {
		jsonError(w, "session not found in context", http.StatusInternalServerError)
		return
	}

	// List all sessions for user
	sessions, err := h.authService.SessionStore.GetSessionByUserID(r.Context(), userID.String())
	if err != nil {
		h.log.Error("failed to list sessions", zap.Error(err))
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Revoke all except current
	revokedCount := 0
	for _, sess := range sessions {
		if sess.ID != currentSessionID {
			if err := h.authService.SessionStore.RevokeSession(r.Context(), sess.ID); err != nil {
				h.log.Warn("failed to revoke session", zap.String("session_id", sess.ID), zap.Error(err))
				// Continue revoking others even if one fails
				continue
			}
			revokedCount++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]int{"revoked": revokedCount})
}
