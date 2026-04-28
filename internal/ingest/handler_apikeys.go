package ingest

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/argusxdr/argus/internal/auth"
	"github.com/google/uuid"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// ---- Request/Response types ----

type createAPIKeyRequest struct {
	Name      string    `json:"name"`
	AppID     *uuid.UUID `json:"app_id"`
	Scopes    []string  `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at"`
}

type createAPIKeyResponse struct {
	ID        uuid.UUID `json:"id"`
	Key       string    `json:"key"`       // full key value (shown only here)
	Prefix    string    `json:"prefix"`    // first 12 chars for display
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type listAPIKeysResponse struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`       // first 12 chars
	Scopes     []string   `json:"scopes"`
	LastUsedAt *time.Time `json:"last_used_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

// ---- Handlers ----

// handleCreateAPIKey creates a new API key for the authenticated user.
// POST /api/v1/api-keys
func (h *QueryHandler) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	if !h.authAvailable() || h.authService.APIKeyStore == nil {
		jsonError(w, "api key service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Get user from context
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req createAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		jsonError(w, "name required", http.StatusBadRequest)
		return
	}

	// Generate API key
	fullKey, prefix, hash, err := auth.GenerateAPIKey()
	if err != nil {
		h.log.Error("failed to generate api key", zap.Error(err))
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Create API key record
	apiKey := &auth.APIKey{
		ID:        uuid.New(),
		UserID:    userID,
		AppID:     req.AppID,
		Name:      req.Name,
		KeyPrefix: prefix,
		KeyHash:   hash,
		Scopes:    req.Scopes,
		ExpiresAt: req.ExpiresAt,
		CreatedAt: time.Now(),
	}

	if err := h.authService.APIKeyStore.Create(r.Context(), apiKey); err != nil {
		h.log.Error("failed to create api key", zap.Error(err))
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Return key value EXACTLY HERE, never elsewhere
	resp := createAPIKeyResponse{
		ID:        apiKey.ID,
		Key:       fullKey, // full key shown only once
		Prefix:    prefix,
		Name:      apiKey.Name,
		CreatedAt: apiKey.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// handleListAPIKeys lists caller's API keys (or admin can list another user's).
// GET /api/v1/api-keys
func (h *QueryHandler) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	if !h.authAvailable() || h.authService.APIKeyStore == nil {
		jsonError(w, "api key service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Get user from context
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Admin can optionally query another user's keys
	claims := auth.GetUserFromContext(r)
	targetUserID := userID
	if userIDParam := r.URL.Query().Get("user_id"); userIDParam != "" && claims != nil && claims.Role == auth.RoleAdmin {
		parsed, err := uuid.Parse(userIDParam)
		if err != nil {
			jsonError(w, "invalid user_id", http.StatusBadRequest)
			return
		}
		targetUserID = parsed
	}

	keys, err := h.authService.APIKeyStore.ListByUser(r.Context(), targetUserID)
	if err != nil {
		h.log.Error("failed to list api keys", zap.Error(err))
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Build response (never include key_hash or full key)
	resp := make([]listAPIKeysResponse, 0)
	if keys != nil {
		for _, k := range keys {
			resp = append(resp, listAPIKeysResponse{
				ID:         k.ID,
				Name:       k.Name,
				Prefix:     k.KeyPrefix,
				Scopes:     k.Scopes,
				LastUsedAt: k.LastUsedAt,
				ExpiresAt:  k.ExpiresAt,
				RevokedAt:  k.RevokedAt,
				CreatedAt:  k.CreatedAt,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// handleRevokeAPIKey revokes an API key.
// DELETE /api/v1/api-keys/{id}
func (h *QueryHandler) handleRevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	if !h.authAvailable() || h.authService.APIKeyStore == nil {
		jsonError(w, "api key service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Get user from context
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse key ID from URL
	idStr := chi.URLParam(r, "id")
	keyID, err := uuid.Parse(idStr)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}

	// Get the key to check ownership
	key, err := h.authService.APIKeyStore.GetByID(r.Context(), keyID)
	if err != nil {
		h.log.Error("failed to get api key", zap.Error(err))
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if key == nil {
		jsonError(w, "api key not found", http.StatusNotFound)
		return
	}

	// Check ownership: owner or admin can revoke
	claims := auth.GetUserFromContext(r)
	if key.UserID != userID && (claims == nil || claims.Role != auth.RoleAdmin) {
		jsonError(w, "forbidden", http.StatusForbidden)
		return
	}

	// Revoke
	if err := h.authService.APIKeyStore.Revoke(r.Context(), keyID); err != nil {
		h.log.Error("failed to revoke api key", zap.Error(err))
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
