package ingest

import (
	"encoding/json"
	"net/http"

	"github.com/argusxdr/argus/internal/detection/engine"
	"go.uber.org/zap"
)

// ServeListRules handles GET /api/v1/rules.
func (h *QueryHandler) ServeListRules(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	rules := []engine.Rule{}
	if h.store != nil {
		rules = h.store.All()
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"rules": rules})
}

// ServeCreateRule handles POST /api/v1/rules.
func (h *QueryHandler) ServeCreateRule(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var rule engine.Rule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "invalid JSON: " + err.Error()})
		return
	}
	if err := rule.Validate(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
		return
	}
	if h.store != nil {
		h.store.Add(rule)
	}

	h.log.Info("rule created", zap.String("rule_id", rule.ID), zap.String("name", rule.Name))
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(rule)
}

// ServeDeleteRule handles DELETE /api/v1/rules/{id}.
func (h *QueryHandler) ServeDeleteRule(w http.ResponseWriter, _ *http.Request, ruleID string) {
	w.Header().Set("Content-Type", "application/json")
	if h.store != nil {
		h.store.Remove(ruleID)
	}
	h.log.Info("rule deleted", zap.String("rule_id", ruleID))
	w.WriteHeader(http.StatusNoContent)
}

// ServeGetRule handles GET /api/v1/rules/{id}.
func (h *QueryHandler) ServeGetRule(w http.ResponseWriter, _ *http.Request, ruleID string) {
	w.Header().Set("Content-Type", "application/json")
	if h.store == nil {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "rule store not configured"})
		return
	}
	for _, rule := range h.store.All() {
		if rule.ID == ruleID {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(rule)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "rule not found"})
}
