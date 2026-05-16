package behaviour

import (
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"
)

// RecentRunSummary is the per-trace summary returned by ServeRecentRuns.
// Used by the TUI run list (Component D) to display the most recent LLM runs
// for a given application.
type RecentRunSummary struct {
	TraceID       string    `json:"trace_id"`
	FirstSeenAt   time.Time `json:"first_seen_at"`
	LastSeenAt    time.Time `json:"last_seen_at"`
	LayersPresent []int32   `json:"layers_present"`
	SignalCount   int32     `json:"signal_count"`
	PeakDeviation float32   `json:"peak_deviation"`
	DurationMS    int64     `json:"duration_ms"`
}

// recentRunsSQL aggregates signals by trace_id to produce the run list.
// groupUniqArray deduplications layer values; the result is ordered by most
// recent activity descending and capped by the caller-supplied limit.
const recentRunsSQL = `
	SELECT
		trace_id,
		min(timestamp)                       AS first_seen,
		max(timestamp)                       AS last_seen,
		groupUniqArray(toInt32(layer))       AS layers,
		toInt32(count())                     AS sig_count,
		max(enrich_baseline_deviation)       AS peak_dev
	FROM signals
	WHERE app_id = ?
	GROUP BY trace_id
	ORDER BY last_seen DESC
	LIMIT ?`

// ServeRecentRuns handles GET /api/v1/traces/recent?app_id=X&limit=N.
//
// Required query param: app_id — returns 400 when absent.
// Optional query param: limit — default 50, capped at 200.
//
// Returns a JSON array of RecentRunSummary, ordered by most-recent-activity
// descending. An empty array (not null) is returned when no runs exist.
func (h *BehaviourHandler) ServeRecentRuns(w http.ResponseWriter, r *http.Request) {
	appID := r.URL.Query().Get("app_id")
	if appID == "" {
		writeErr(w, http.StatusBadRequest, "app_id required")
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	rows, err := h.ch.Query(r.Context(), recentRunsSQL, appID, limit)
	if err != nil {
		h.logger.Error("recent runs query failed",
			zap.String("app_id", appID), zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "query failed")
		return
	}
	defer rows.Close()

	out := []RecentRunSummary{} // initialise to non-nil so JSON encodes as []
	for rows.Next() {
		var s RecentRunSummary
		var peak *float32
		if err := rows.Scan(
			&s.TraceID,
			&s.FirstSeenAt,
			&s.LastSeenAt,
			&s.LayersPresent,
			&s.SignalCount,
			&peak,
		); err != nil {
			h.logger.Warn("recent runs scan failed", zap.Error(err))
			continue
		}
		if peak != nil {
			s.PeakDeviation = *peak
		}
		s.DurationMS = s.LastSeenAt.Sub(s.FirstSeenAt).Milliseconds()
		out = append(out, s)
	}
	writeJSON(w, http.StatusOK, out)
}
