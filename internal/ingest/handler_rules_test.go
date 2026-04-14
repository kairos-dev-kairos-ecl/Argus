package ingest_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/argusxdr/argus/internal/detection/engine"
	"github.com/argusxdr/argus/internal/ingest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestHandleListRulesEmpty(t *testing.T) {
	store := engine.NewRuleStore()
	h := ingest.NewQueryHandler(nil, nil, zap.NewNop())
	h.SetRuleStore(store)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rules", nil)
	w := httptest.NewRecorder()
	h.ServeListRules(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	rules, ok := resp["rules"].([]interface{})
	assert.True(t, ok)
	assert.Empty(t, rules)
}

func TestHandleCreateRuleValid(t *testing.T) {
	store := engine.NewRuleStore()
	h := ingest.NewQueryHandler(nil, nil, zap.NewNop())
	h.SetRuleStore(store)

	body := map[string]interface{}{
		"id":       "test-r1",
		"name":     "Test Rule",
		"tier":     1,
		"enabled":  true,
		"severity": 3,
		"conditions": map[string]interface{}{
			"layer": "L6_SAFETY",
		},
		"action": map[string]interface{}{
			"title": "Test Alert",
		},
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rules", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeCreateRule(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, 1, store.Count())
}

func TestHandleCreateRuleInvalidBody(t *testing.T) {
	store := engine.NewRuleStore()
	h := ingest.NewQueryHandler(nil, nil, zap.NewNop())
	h.SetRuleStore(store)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/rules", bytes.NewReader([]byte(`{invalid}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeCreateRule(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleDeleteRule(t *testing.T) {
	store := engine.NewRuleStore()
	store.Add(engine.Rule{
		ID: "del-r1", Name: "R", Tier: 1, Enabled: true, Severity: 3,
		Action: engine.Action{Title: "x"},
	})
	h := ingest.NewQueryHandler(nil, nil, zap.NewNop())
	h.SetRuleStore(store)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/rules/del-r1", nil)
	w := httptest.NewRecorder()
	h.ServeDeleteRule(w, req, "del-r1")

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, 0, store.Count())
}
