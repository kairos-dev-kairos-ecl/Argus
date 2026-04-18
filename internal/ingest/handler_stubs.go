package ingest

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/argusxdr/argus/internal/detection/engine"
	"go.uber.org/zap"
)

// Rules handlers — DB-backed via detection_rules PostgreSQL table.
// CRITICAL: These do NOT delegate to ServeListRules/ServeCreateRule/ServeGetRule/ServeDeleteRule
// (those are in-memory-only and would lose state on restart).

func (h *QueryHandler) handleListRules(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.pool == nil {
		jsonError(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	rows, err := h.pool.Query(r.Context(),
		`SELECT id, name, description, tier, enabled, config, created_at, updated_at FROM detection_rules ORDER BY created_at DESC`)
	if err != nil {
		h.log.Error("list rules query failed", zap.Error(err))
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	type RuleView struct {
		ID          string          `json:"id"`
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Tier        int             `json:"tier"`
		Enabled     bool            `json:"enabled"`
		Config      json.RawMessage `json:"config"`
		CreatedAt   string          `json:"created_at"`
		UpdatedAt   string          `json:"updated_at"`
	}
	rules := []RuleView{}
	for rows.Next() {
		var rv RuleView
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&rv.ID, &rv.Name, &rv.Description, &rv.Tier, &rv.Enabled, &rv.Config, &createdAt, &updatedAt); err != nil {
			h.log.Warn("scan rule row failed", zap.Error(err))
			continue
		}
		rv.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		rv.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
		rules = append(rules, rv)
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"rules": rules})
}

func (h *QueryHandler) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.pool == nil {
		jsonError(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	var rule engine.Rule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		jsonError(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}
	if err := rule.Validate(); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	configJSON, _ := json.Marshal(rule)
	_, err := h.pool.Exec(r.Context(),
		`INSERT INTO detection_rules (id, name, description, tier, enabled, config, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())`,
		rule.ID, rule.Name, rule.Description, rule.Tier, rule.Enabled, configJSON)
	if err != nil {
		h.log.Error("create rule DB insert failed", zap.Error(err))
		jsonError(w, "create failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// Sync in-memory store for live detection (post-DB success only)
	if h.store != nil {
		h.store.Add(rule)
	}
	h.log.Info("rule created", zap.String("rule_id", rule.ID))
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(rule)
}

func (h *QueryHandler) handleGetRule(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.pool == nil {
		jsonError(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	ruleID := chi.URLParam(r, "id")
	var configJSON []byte
	err := h.pool.QueryRow(r.Context(),
		`SELECT config FROM detection_rules WHERE id = $1`, ruleID).Scan(&configJSON)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "rule not found"})
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(configJSON)
}

func (h *QueryHandler) handleUpdateRule(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.pool == nil {
		jsonError(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	ruleID := chi.URLParam(r, "id")
	var rule engine.Rule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		jsonError(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	rule.ID = ruleID
	if err := rule.Validate(); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	configJSON, _ := json.Marshal(rule)
	tag, err := h.pool.Exec(r.Context(),
		`UPDATE detection_rules SET name=$1, description=$2, tier=$3, enabled=$4, config=$5, updated_at=NOW() WHERE id=$6`,
		rule.Name, rule.Description, rule.Tier, rule.Enabled, configJSON, ruleID)
	if err != nil {
		h.log.Error("update rule DB failed", zap.Error(err))
		jsonError(w, "update failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "rule not found"})
		return
	}
	// Sync in-memory store (Remove + Add since no Update method)
	if h.store != nil {
		h.store.Remove(ruleID)
		h.store.Add(rule)
	}
	h.log.Info("rule updated", zap.String("rule_id", ruleID))
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(rule)
}

func (h *QueryHandler) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.pool == nil {
		jsonError(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	ruleID := chi.URLParam(r, "id")
	_, err := h.pool.Exec(r.Context(),
		`DELETE FROM detection_rules WHERE id=$1`, ruleID)
	if err != nil {
		h.log.Error("delete rule DB failed", zap.Error(err))
		jsonError(w, "delete failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if h.store != nil {
		h.store.Remove(ruleID)
	}
	h.log.Info("rule deleted", zap.String("rule_id", ruleID))
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"deleted": ruleID})
}

func (h *QueryHandler) handleValidateRule(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var rule engine.Rule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		jsonError(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := rule.Validate(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"valid": false, "errors": []string{err.Error()}})
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"valid": true, "errors": []string{}})
}

func (h *QueryHandler) handleTestRule(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"matches": []interface{}{}, "tested_count": 0})
}
