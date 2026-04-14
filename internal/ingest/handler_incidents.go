package ingest

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

func (h *QueryHandler) handleListIncidents(w http.ResponseWriter, r *http.Request) {
	if !h.alertAvailable() {
		jsonError(w, "alert pipeline unavailable", http.StatusServiceUnavailable)
		return
	}

	status := r.URL.Query().Get("status")
	limit := 100
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}

	query := `
		SELECT id, title, description, severity, app_id, status, alert_ids, trace_ids,
		       created_at, updated_at, acknowledged_at, resolved_at
		FROM incidents WHERE 1=1`
	args := []interface{}{}
	argIdx := 1
	if status != "" {
		query += ` AND status = $` + strconv.Itoa(argIdx)
		args = append(args, status)
		argIdx++
	}
	query += ` ORDER BY created_at DESC LIMIT $` + strconv.Itoa(argIdx)
	args = append(args, limit)

	rows, err := h.alertRouter.pool.Query(r.Context(), query, args...)
	if err != nil {
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type incidentRow struct {
		ID             string     `json:"id"`
		Title          string     `json:"title"`
		Description    *string    `json:"description,omitempty"`
		Severity       int        `json:"severity"`
		AppID          string     `json:"app_id"`
		Status         string     `json:"status"`
		AlertIDs       []string   `json:"alert_ids"`
		TraceIDs       []string   `json:"trace_ids"`
		CreatedAt      time.Time  `json:"created_at"`
		UpdatedAt      time.Time  `json:"updated_at"`
		AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
		ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	}

	incidents := []incidentRow{}
	for rows.Next() {
		var inc incidentRow
		if err := rows.Scan(
			&inc.ID, &inc.Title, &inc.Description, &inc.Severity, &inc.AppID, &inc.Status,
			&inc.AlertIDs, &inc.TraceIDs, &inc.CreatedAt, &inc.UpdatedAt, &inc.AcknowledgedAt, &inc.ResolvedAt,
		); err != nil {
			continue
		}
		incidents = append(incidents, inc)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"incidents": incidents, "count": len(incidents)})
}

func (h *QueryHandler) handleGetIncident(w http.ResponseWriter, r *http.Request) {
	if !h.alertAvailable() {
		jsonError(w, "alert pipeline unavailable", http.StatusServiceUnavailable)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		jsonError(w, "missing id", http.StatusBadRequest)
		return
	}

	type incidentDetail struct {
		ID             string     `json:"id"`
		Title          string     `json:"title"`
		Description    *string    `json:"description,omitempty"`
		Severity       int        `json:"severity"`
		AppID          string     `json:"app_id"`
		Status         string     `json:"status"`
		AlertIDs       []string   `json:"alert_ids"`
		TraceIDs       []string   `json:"trace_ids"`
		CreatedAt      time.Time  `json:"created_at"`
		UpdatedAt      time.Time  `json:"updated_at"`
		AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
		ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	}

	var inc incidentDetail
	err := h.alertRouter.pool.QueryRow(r.Context(), `
		SELECT id, title, description, severity, app_id, status, alert_ids, trace_ids,
		       created_at, updated_at, acknowledged_at, resolved_at
		FROM incidents WHERE id = $1
	`, id).Scan(
		&inc.ID, &inc.Title, &inc.Description, &inc.Severity, &inc.AppID, &inc.Status,
		&inc.AlertIDs, &inc.TraceIDs, &inc.CreatedAt, &inc.UpdatedAt, &inc.AcknowledgedAt, &inc.ResolvedAt,
	)
	if err != nil {
		jsonError(w, "incident not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(inc)
}

func (h *QueryHandler) handleAcknowledgeIncident(w http.ResponseWriter, r *http.Request) {
	if !h.alertAvailable() {
		jsonError(w, "alert pipeline unavailable", http.StatusServiceUnavailable)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		jsonError(w, "missing id", http.StatusBadRequest)
		return
	}

	result, err := h.alertRouter.pool.Exec(r.Context(), `
		UPDATE incidents
		SET status = 'acknowledged', acknowledged_at = now(), updated_at = now()
		WHERE id = $1 AND status = 'open'
	`, id)
	if err != nil {
		jsonError(w, "update failed", http.StatusInternalServerError)
		return
	}
	if result.RowsAffected() == 0 {
		jsonError(w, "incident not found or already acknowledged", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "acknowledged"})
}

func (h *QueryHandler) handleResolveIncident(w http.ResponseWriter, r *http.Request) {
	if !h.alertAvailable() {
		jsonError(w, "alert pipeline unavailable", http.StatusServiceUnavailable)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		jsonError(w, "missing id", http.StatusBadRequest)
		return
	}

	result, err := h.alertRouter.pool.Exec(r.Context(), `
		UPDATE incidents
		SET status = 'resolved', resolved_at = now(), updated_at = now()
		WHERE id = $1 AND status != 'resolved'
	`, id)
	if err != nil {
		jsonError(w, "update failed", http.StatusInternalServerError)
		return
	}
	if result.RowsAffected() == 0 {
		jsonError(w, "incident not found or already resolved", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "resolved"})
}
