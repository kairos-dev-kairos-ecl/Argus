// Package behaviour provides HTTP handlers for the Phase 7 behavioural
// traceability endpoints: trace graph, session timeline, conversation
// behaviour (with drift scoring), alert causal chain, and recent run list.
package behaviour

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/argusxdr/argus/internal/baseline"
	"github.com/argusxdr/argus/internal/trace"
)

// BehaviourHandler owns the five Phase-7 API endpoints. All methods satisfy
// http.HandlerFunc and must be registered under RequireAuth + RequireRole middleware.
type BehaviourHandler struct {
	ch              driver.Conn
	pg              *pgxpool.Pool
	reconstructor   *trace.RunReconstructor
	timelineBuilder *trace.TimelineBuilder
	profileStore    *baseline.SessionProfileStore
	logger          *zap.Logger
}

// NewBehaviourHandler constructs a BehaviourHandler with all dependencies injected.
// If log is nil a no-op logger is used.
func NewBehaviourHandler(
	ch driver.Conn,
	pg *pgxpool.Pool,
	rec *trace.RunReconstructor,
	tb *trace.TimelineBuilder,
	ps *baseline.SessionProfileStore,
	log *zap.Logger,
) *BehaviourHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &BehaviourHandler{
		ch:              ch,
		pg:              pg,
		reconstructor:   rec,
		timelineBuilder: tb,
		profileStore:    ps,
		logger:          log,
	}
}

// RegisterRoutes mounts all five behaviour endpoints on the given router.
// The caller is responsible for wrapping this group with RequireAuth and
// RequireRole("analyst","admin") before calling RegisterRoutes.
func (h *BehaviourHandler) RegisterRoutes(r chi.Router) {
	r.Get("/api/v1/traces/recent", h.ServeRecentRuns)
	r.Get("/api/v1/traces/{traceID}/graph", h.ServeTraceGraph)
	r.Get("/api/v1/sessions/{sessionID}/timeline", h.ServeSessionTimeline)
	r.Get("/api/v1/conversations/{conversationID}/behaviour", h.ServeConversationBehaviour)
	r.Get("/api/v1/alerts/{alertID}/chain", h.ServeAlertChain)
}

// ServeTraceGraph handles GET /api/v1/traces/{traceID}/graph.
// Returns the full causal RunGraph for the given trace, or 404 when no signals
// exist for that trace_id.
func (h *BehaviourHandler) ServeTraceGraph(w http.ResponseWriter, r *http.Request) {
	traceID := chi.URLParam(r, "traceID")
	if traceID == "" {
		writeErr(w, http.StatusBadRequest, "traceID required")
		return
	}
	graph, err := h.reconstructor.BuildRun(r.Context(), traceID)
	if errors.Is(err, trace.ErrTraceNotFound) {
		writeErr(w, http.StatusNotFound, "trace not found")
		return
	}
	if err != nil {
		h.logger.Error("trace graph build failed", zap.String("trace_id", traceID), zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "build failed")
		return
	}
	writeJSON(w, http.StatusOK, graph)
}

// ServeSessionTimeline handles GET /api/v1/sessions/{sessionID}/timeline.
// Returns the ordered SessionTimeline for all signals sharing the given session_id.
func (h *BehaviourHandler) ServeSessionTimeline(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	if sessionID == "" {
		writeErr(w, http.StatusBadRequest, "sessionID required")
		return
	}
	tl, err := h.timelineBuilder.BuildFromSession(r.Context(), sessionID)
	if err != nil {
		h.logger.Error("session timeline build failed", zap.String("session_id", sessionID), zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "build failed")
		return
	}
	if tl == nil || len(tl.Events) == 0 {
		writeErr(w, http.StatusNotFound, "no signals for session")
		return
	}
	writeJSON(w, http.StatusOK, tl)
}

// conversationBehaviourResponse is the JSON envelope returned by ServeConversationBehaviour.
type conversationBehaviourResponse struct {
	Timeline        *trace.SessionTimeline   `json:"timeline"`
	SessionBaseline *baseline.SessionProfile `json:"session_baseline"`
	DriftScore      *float64                 `json:"drift_score"`
	AppID           string                   `json:"app_id"`
}

// ServeConversationBehaviour handles GET /api/v1/conversations/{conversationID}/behaviour.
// Returns the SessionTimeline plus drift_score (normalised Levenshtein distance) when a
// session baseline profile exists for the conversation's app_id. drift_score is null when
// no profile is available — this is an acceptable first-pass state per RESEARCH.md.
func (h *BehaviourHandler) ServeConversationBehaviour(w http.ResponseWriter, r *http.Request) {
	convID := chi.URLParam(r, "conversationID")
	if convID == "" {
		writeErr(w, http.StatusBadRequest, "conversationID required")
		return
	}
	tl, err := h.timelineBuilder.BuildFromConversation(r.Context(), convID)
	if err != nil {
		h.logger.Error("conversation timeline build failed", zap.String("conversation_id", convID), zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "build failed")
		return
	}
	if tl == nil || len(tl.Events) == 0 {
		writeErr(w, http.StatusNotFound, "no signals for conversation")
		return
	}

	// Resolve app_id from ClickHouse; non-fatal if lookup fails.
	appID, lookupErr := h.lookupAppIDForConversation(r.Context(), convID)
	if lookupErr != nil {
		h.logger.Warn("app_id lookup failed for conversation",
			zap.String("conversation_id", convID), zap.Error(lookupErr))
	}

	resp := conversationBehaviourResponse{
		Timeline: tl,
		AppID:    appID,
	}

	// Compute drift only when we have an app_id and a stored baseline.
	if appID != "" {
		prof, profErr := h.profileStore.Get(r.Context(), appID)
		if profErr != nil {
			h.logger.Warn("session profile lookup failed",
				zap.String("app_id", appID), zap.Error(profErr))
		}
		if prof != nil {
			resp.SessionBaseline = prof
			d := baseline.ComputeSessionDrift(
				tl.Aggregates.LayerActivationSequence,
				prof.LayerActivationSequence,
			)
			resp.DriftScore = &d
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// lookupAppIDForConversation fetches the app_id of the first signal for convID.
// Returns ("", nil) when no signal is found — drift will be skipped.
func (h *BehaviourHandler) lookupAppIDForConversation(ctx context.Context, convID string) (string, error) {
	const q = `SELECT app_id FROM signals WHERE conversation_id = ? ORDER BY timestamp ASC LIMIT 1`
	rows, err := h.ch.Query(ctx, q, convID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	if !rows.Next() {
		return "", nil
	}
	var appID string
	if err := rows.Scan(&appID); err != nil {
		return "", err
	}
	return appID, nil
}

// ServeAlertChain handles GET /api/v1/alerts/{alertID}/chain.
// Fetches the alert's signal_ids from PostgreSQL, resolves the trace_id of the first
// signal from ClickHouse, and returns the full RunGraph for that trace.
// Per RESEARCH.md partial-success definition, ancestor-only filtering is deferred.
func (h *BehaviourHandler) ServeAlertChain(w http.ResponseWriter, r *http.Request) {
	alertID := chi.URLParam(r, "alertID")
	if alertID == "" {
		writeErr(w, http.StatusBadRequest, "alertID required")
		return
	}

	// Fetch signal_ids from PostgreSQL alerts table.
	var signalIDs []string
	err := h.pg.QueryRow(r.Context(),
		`SELECT signal_ids FROM alerts WHERE id = $1`, alertID).Scan(&signalIDs)
	if err != nil {
		writeErr(w, http.StatusNotFound, "alert not found")
		return
	}
	if len(signalIDs) == 0 {
		writeErr(w, http.StatusNotFound, "alert has no signals")
		return
	}

	// Resolve trace_id from ClickHouse for the first signal.
	var traceID string
	rows, err := h.ch.Query(r.Context(),
		`SELECT trace_id FROM signals WHERE signal_id = ? LIMIT 1`, signalIDs[0])
	if err != nil {
		h.logger.Error("trace_id lookup failed", zap.String("signal_id", signalIDs[0]), zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "lookup failed")
		return
	}
	if rows.Next() {
		_ = rows.Scan(&traceID)
	}
	rows.Close()

	if traceID == "" {
		writeErr(w, http.StatusNotFound, "no trace for alert signal")
		return
	}

	graph, err := h.reconstructor.BuildRun(r.Context(), traceID)
	if errors.Is(err, trace.ErrTraceNotFound) {
		writeErr(w, http.StatusNotFound, "trace not found")
		return
	}
	if err != nil {
		h.logger.Error("alert chain build failed", zap.String("alert_id", alertID), zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "build failed")
		return
	}
	writeJSON(w, http.StatusOK, graph)
}

// writeJSON serialises v as JSON and writes it with the given HTTP status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeErr writes a {"error": msg} JSON body with the given status code.
func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
