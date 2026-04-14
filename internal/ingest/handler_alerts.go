package ingest

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

func (h *QueryHandler) alertAvailable() bool {
	return h.alertRouter != nil && h.alertRouter.pool != nil
}

func (h *QueryHandler) handleListAlerts(w http.ResponseWriter, r *http.Request) {
	if !h.alertAvailable() {
		jsonError(w, "alert pipeline unavailable", http.StatusServiceUnavailable)
		return
	}

	status := r.URL.Query().Get("status")
	appID := r.URL.Query().Get("app_id")
	limit := 100
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}

	query := `
		SELECT id, app_id, fingerprint, severity, layer, category, title, description,
		       signal_ids, trace_id, incident_id, status, signal_count,
		       first_seen_at, last_seen_at, acknowledged_at, resolved_at
		FROM alerts
		WHERE 1=1`
	args := []interface{}{}
	argIdx := 1
	if status != "" {
		query += ` AND status = $` + strconv.Itoa(argIdx)
		args = append(args, status)
		argIdx++
	}
	if appID != "" {
		query += ` AND app_id = $` + strconv.Itoa(argIdx)
		args = append(args, appID)
		argIdx++
	}
	query += ` ORDER BY first_seen_at DESC LIMIT $` + strconv.Itoa(argIdx)
	args = append(args, limit)

	rows, err := h.alertRouter.pool.Query(r.Context(), query, args...)
	if err != nil {
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type alertRow struct {
		ID             string     `json:"id"`
		AppID          string     `json:"app_id"`
		Fingerprint    string     `json:"fingerprint"`
		Severity       int        `json:"severity"`
		Layer          int        `json:"layer"`
		Category       string     `json:"category"`
		Title          string     `json:"title"`
		Description    *string    `json:"description,omitempty"`
		SignalIDs      []string   `json:"signal_ids"`
		TraceID        *string    `json:"trace_id,omitempty"`
		IncidentID     *string    `json:"incident_id,omitempty"`
		Status         string     `json:"status"`
		SignalCount    int        `json:"signal_count"`
		FirstSeenAt    time.Time  `json:"first_seen_at"`
		LastSeenAt     time.Time  `json:"last_seen_at"`
		AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
		ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	}

	alerts := []alertRow{}
	for rows.Next() {
		var row alertRow
		if err := rows.Scan(
			&row.ID, &row.AppID, &row.Fingerprint, &row.Severity, &row.Layer, &row.Category,
			&row.Title, &row.Description, &row.SignalIDs, &row.TraceID, &row.IncidentID, &row.Status,
			&row.SignalCount, &row.FirstSeenAt, &row.LastSeenAt, &row.AcknowledgedAt, &row.ResolvedAt,
		); err != nil {
			continue
		}
		alerts = append(alerts, row)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"alerts": alerts, "count": len(alerts)})
}

func (h *QueryHandler) handleGetAlert(w http.ResponseWriter, r *http.Request) {
	if !h.alertAvailable() {
		jsonError(w, "alert pipeline unavailable", http.StatusServiceUnavailable)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		jsonError(w, "missing id", http.StatusBadRequest)
		return
	}

	type alertDetail struct {
		ID             string     `json:"id"`
		AppID          string     `json:"app_id"`
		Fingerprint    string     `json:"fingerprint"`
		Severity       int        `json:"severity"`
		Layer          int        `json:"layer"`
		Category       string     `json:"category"`
		Title          string     `json:"title"`
		Description    *string    `json:"description,omitempty"`
		SignalIDs      []string   `json:"signal_ids"`
		TraceID        *string    `json:"trace_id,omitempty"`
		IncidentID     *string    `json:"incident_id,omitempty"`
		Status         string     `json:"status"`
		SignalCount    int        `json:"signal_count"`
		FirstSeenAt    time.Time  `json:"first_seen_at"`
		LastSeenAt     time.Time  `json:"last_seen_at"`
		AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
		ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	}

	var a alertDetail
	err := h.alertRouter.pool.QueryRow(r.Context(), `
		SELECT id, app_id, fingerprint, severity, layer, category, title, description,
		       signal_ids, trace_id, incident_id, status, signal_count,
		       first_seen_at, last_seen_at, acknowledged_at, resolved_at
		FROM alerts WHERE id = $1
	`, id).Scan(
		&a.ID, &a.AppID, &a.Fingerprint, &a.Severity, &a.Layer, &a.Category,
		&a.Title, &a.Description, &a.SignalIDs, &a.TraceID, &a.IncidentID, &a.Status, &a.SignalCount,
		&a.FirstSeenAt, &a.LastSeenAt, &a.AcknowledgedAt, &a.ResolvedAt,
	)
	if err != nil {
		jsonError(w, "alert not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(a)
}

func (h *QueryHandler) handleAcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
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
		UPDATE alerts
		SET status = 'acknowledged', acknowledged_at = now()
		WHERE id = $1 AND status = 'open'
	`, id)
	if err != nil {
		jsonError(w, "update failed", http.StatusInternalServerError)
		return
	}
	if result.RowsAffected() == 0 {
		jsonError(w, "alert not found or already acknowledged", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "acknowledged"})
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: msg})
}
