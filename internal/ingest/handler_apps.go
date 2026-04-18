package ingest

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func (h *QueryHandler) handleListApps(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.pool == nil {
		jsonError(w, "storage unavailable", http.StatusServiceUnavailable)
		return
	}
	rows, err := h.pool.Query(r.Context(),
		`SELECT id, name, description, api_key_prefix, created_at, updated_at FROM apps ORDER BY created_at DESC`)
	if err != nil {
		h.log.Error("list apps query failed", zap.Error(err))
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type AppView struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		Description  string `json:"description"`
		APIKeyPrefix string `json:"api_key_prefix"`
		CreatedAt    string `json:"created_at"`
		UpdatedAt    string `json:"updated_at"`
	}
	apps := []AppView{}
	for rows.Next() {
		var a AppView
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&a.ID, &a.Name, &a.Description, &a.APIKeyPrefix, &createdAt, &updatedAt); err != nil {
			h.log.Warn("scan app row failed", zap.Error(err))
			continue
		}
		a.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		a.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
		apps = append(apps, a)
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"apps": apps})
}

func (h *QueryHandler) handleCreateApp(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.pool == nil {
		jsonError(w, "storage unavailable", http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}

	// Generate API key: "arg_" + 32 random bytes hex = 68 char key
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		jsonError(w, "key generation failed", http.StatusInternalServerError)
		return
	}
	plainKey := "arg_" + hex.EncodeToString(keyBytes)
	prefix := plainKey[:12] // "arg_" + first 8 hex chars

	// SHA256 hash for storage
	hash := sha256.Sum256([]byte(plainKey))
	hashHex := hex.EncodeToString(hash[:])

	appID := uuid.New().String()
	_, err := h.pool.Exec(r.Context(),
		`INSERT INTO apps (id, name, description, api_key, api_key_prefix, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, NOW(), NOW())`,
		appID, req.Name, req.Description, hashHex, prefix)
	if err != nil {
		h.log.Error("create app failed", zap.Error(err))
		jsonError(w, "create failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.log.Info("app created", zap.String("app_id", appID), zap.String("name", req.Name))
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"id":             appID,
		"name":           req.Name,
		"description":    req.Description,
		"api_key":        plainKey, // returned ONCE at creation
		"api_key_prefix": prefix,
	})
}

func (h *QueryHandler) handleGetAppKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.pool == nil {
		jsonError(w, "storage unavailable", http.StatusServiceUnavailable)
		return
	}
	appID := chi.URLParam(r, "id")
	var prefix string
	err := h.pool.QueryRow(r.Context(),
		`SELECT api_key_prefix FROM apps WHERE id = $1`, appID).Scan(&prefix)
	if err != nil {
		jsonError(w, "app not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"id":             appID,
		"api_key_prefix": prefix,
	})
}

func (h *QueryHandler) handleRotateAppKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.pool == nil {
		jsonError(w, "storage unavailable", http.StatusServiceUnavailable)
		return
	}
	appID := chi.URLParam(r, "id")

	// Generate new key
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		jsonError(w, "key generation failed", http.StatusInternalServerError)
		return
	}
	plainKey := "arg_" + hex.EncodeToString(keyBytes)
	prefix := plainKey[:12]
	hash := sha256.Sum256([]byte(plainKey))
	hashHex := hex.EncodeToString(hash[:])

	tag, err := h.pool.Exec(r.Context(),
		`UPDATE apps SET api_key = $1, api_key_prefix = $2, updated_at = NOW() WHERE id = $3`,
		hashHex, prefix, appID)
	if err != nil || tag.RowsAffected() == 0 {
		jsonError(w, "app not found or update failed", http.StatusNotFound)
		return
	}

	h.log.Info("app key rotated", zap.String("app_id", appID))
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"id":             appID,
		"api_key":        plainKey, // returned ONCE
		"api_key_prefix": prefix,
	})
}
