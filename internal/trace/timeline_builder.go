package trace

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// TimelineBuilder reconstructs session-scoped or conversation-scoped signal timelines
// from ClickHouse. It is a query-time component — not a pipeline stage.
type TimelineBuilder struct {
	ch driver.Conn
}

// NewTimelineBuilder creates a TimelineBuilder backed by the given ClickHouse connection.
func NewTimelineBuilder(ch driver.Conn) *TimelineBuilder {
	return &TimelineBuilder{ch: ch}
}

// BuildFromSession returns a SessionTimeline for all signals with the given session_id.
func (t *TimelineBuilder) BuildFromSession(ctx context.Context, sessionID string) (*SessionTimeline, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("sessionID required")
	}
	return t.build(ctx, "session", sessionID, SelectSessionSignalsSQLBySessionID)
}

// BuildFromConversation returns a SessionTimeline for all signals with the given conversation_id.
func (t *TimelineBuilder) BuildFromConversation(ctx context.Context, conversationID string) (*SessionTimeline, error) {
	if conversationID == "" {
		return nil, fmt.Errorf("conversationID required")
	}
	return t.build(ctx, "conversation", conversationID, SelectSessionSignalsSQLByConversationID)
}

func (t *TimelineBuilder) build(ctx context.Context, scopeKind, scopeID, query string) (*SessionTimeline, error) {
	rows, err := t.ch.Query(ctx, query, scopeID)
	if err != nil {
		return nil, fmt.Errorf("query session: %w", err)
	}
	defer rows.Close()

	var events []*TimelineEvent
	for rows.Next() {
		ev, err := scanTimelineEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return assembleTimeline(scopeKind, scopeID, events), nil
}

// scanTimelineEvent scans one row from SelectSessionSignalsSQLBy* into a TimelineEvent.
// Column order mirrors scanRunNode; the LayerLabel and IsAnomaly flag are set inline.
func scanTimelineEvent(rows driver.Rows) (*TimelineEvent, error) {
	var (
		signalID     string
		spanID       string
		parentSpanID sql.NullString // scanned but not used in TimelineEvent
		layer        uint8
		category     string
		severity     uint8
		timestamp    time.Time
		durationMS   sql.NullFloat64
		baselineDev  sql.NullFloat64

		// L3
		l3InputTokens  sql.NullInt64
		l3OutputTokens sql.NullInt64
		l3TokenEntropy sql.NullFloat64

		// L4
		l4KVCacheHitRate sql.NullFloat64
		l4DecodeTimeMS   sql.NullFloat64

		// L5
		l5Temperature  sql.NullFloat64
		l5MeanLogprob  sql.NullFloat64
		l5EntropyMean  sql.NullFloat64
		l5FinishReason sql.NullString

		// L6
		l6SafetyScore          sql.NullFloat64
		l6JailbreakScore       sql.NullFloat64
		l6PromptInjectionScore sql.NullFloat64

		// L7
		l7ResultsCount    sql.NullInt64
		l7LatencySearchMS sql.NullFloat64
		l7QueryCacheHit   sql.NullInt64

		// L8
		l8ToolName      sql.NullString
		l8ToolLatencyMS sql.NullFloat64
		l8StepNumber    sql.NullInt64

		// L9
		l9Method     sql.NullString
		l9Path       sql.NullString
		l9StatusCode sql.NullInt64
		l9LatencyMS  sql.NullFloat64

		// L10
		l10UserID         sql.NullString
		l10SessionID      sql.NullString
		l10ConversationID sql.NullString
		l10EventType      sql.NullString
	)

	err := rows.Scan(
		&signalID,
		&spanID,
		&parentSpanID,
		&layer,
		&category,
		&severity,
		&timestamp,
		&durationMS,
		&baselineDev,
		&l3InputTokens,
		&l3OutputTokens,
		&l3TokenEntropy,
		&l4KVCacheHitRate,
		&l4DecodeTimeMS,
		&l5Temperature,
		&l5MeanLogprob,
		&l5EntropyMean,
		&l5FinishReason,
		&l6SafetyScore,
		&l6JailbreakScore,
		&l6PromptInjectionScore,
		&l7ResultsCount,
		&l7LatencySearchMS,
		&l7QueryCacheHit,
		&l8ToolName,
		&l8ToolLatencyMS,
		&l8StepNumber,
		&l9Method,
		&l9Path,
		&l9StatusCode,
		&l9LatencyMS,
		&l10UserID,
		&l10SessionID,
		&l10ConversationID,
		&l10EventType,
	)
	if err != nil {
		return nil, err
	}

	ev := &TimelineEvent{
		SignalID:   signalID,
		Timestamp:  timestamp,
		Layer:      int32(layer),
		LayerLabel: fmt.Sprintf("L%d", layer),
		Category:   category,
		Severity:   int32(severity),
	}
	if durationMS.Valid {
		ev.DurationMS = float32(durationMS.Float64)
	}
	if baselineDev.Valid {
		ev.BaselineDeviation = float32(baselineDev.Float64)
	}
	ev.IsAnomaly = ev.BaselineDeviation > AnomalyDeviationThreshold

	// Build CtxSummary with non-null context fields.
	ctxMap := make(map[string]interface{})
	if l3InputTokens.Valid {
		ctxMap["l3_input_tokens"] = l3InputTokens.Int64
	}
	if l3OutputTokens.Valid {
		ctxMap["l3_output_tokens"] = l3OutputTokens.Int64
	}
	if l3TokenEntropy.Valid {
		ctxMap["l3_token_entropy"] = l3TokenEntropy.Float64
	}
	if l4KVCacheHitRate.Valid {
		ctxMap["l4_kv_cache_hit_rate"] = l4KVCacheHitRate.Float64
	}
	if l4DecodeTimeMS.Valid {
		ctxMap["l4_decode_time_ms"] = l4DecodeTimeMS.Float64
	}
	if l5Temperature.Valid {
		ctxMap["l5_temperature"] = l5Temperature.Float64
	}
	if l5MeanLogprob.Valid {
		ctxMap["l5_mean_logprob"] = l5MeanLogprob.Float64
	}
	if l5EntropyMean.Valid {
		ctxMap["l5_entropy_mean"] = l5EntropyMean.Float64
	}
	if l5FinishReason.Valid {
		ctxMap["l5_finish_reason"] = l5FinishReason.String
	}
	if l6SafetyScore.Valid {
		ctxMap["l6_safety_score"] = l6SafetyScore.Float64
	}
	if l6JailbreakScore.Valid {
		ctxMap["l6_jailbreak_score"] = l6JailbreakScore.Float64
	}
	if l6PromptInjectionScore.Valid {
		ctxMap["l6_prompt_injection_score"] = l6PromptInjectionScore.Float64
	}
	if l7ResultsCount.Valid {
		ctxMap["l7_results_count"] = l7ResultsCount.Int64
	}
	if l7LatencySearchMS.Valid {
		ctxMap["l7_latency_search_ms"] = l7LatencySearchMS.Float64
	}
	if l7QueryCacheHit.Valid {
		ctxMap["l7_query_cache_hit"] = l7QueryCacheHit.Int64 != 0
	}
	if l8ToolName.Valid {
		ctxMap["l8_tool_name"] = l8ToolName.String
	}
	if l8ToolLatencyMS.Valid {
		ctxMap["l8_tool_latency_ms"] = l8ToolLatencyMS.Float64
	}
	if l8StepNumber.Valid {
		ctxMap["l8_step_number"] = l8StepNumber.Int64
	}
	if l9Method.Valid {
		ctxMap["l9_method"] = l9Method.String
	}
	if l9Path.Valid {
		ctxMap["l9_path"] = l9Path.String
	}
	if l9StatusCode.Valid {
		ctxMap["l9_status_code"] = l9StatusCode.Int64
	}
	if l9LatencyMS.Valid {
		ctxMap["l9_latency_ms"] = l9LatencyMS.Float64
	}
	if l10UserID.Valid {
		ctxMap["l10_user_id"] = l10UserID.String
	}
	if l10SessionID.Valid {
		ctxMap["l10_session_id"] = l10SessionID.String
	}
	if l10ConversationID.Valid {
		ctxMap["l10_conversation_id"] = l10ConversationID.String
	}
	if l10EventType.Valid {
		ctxMap["l10_event_type"] = l10EventType.String
	}
	if len(ctxMap) > 0 {
		ev.CtxSummary = ctxMap
	}
	// Suppress unused variable warning for spanID and parentSpanID
	_ = spanID
	_ = parentSpanID
	return ev, nil
}

// assembleTimeline builds a SessionTimeline from an already-sorted slice of TimelineEvents.
// Events must be in timestamp ASC order (guaranteed by the SQL ORDER BY).
func assembleTimeline(scopeKind, scopeID string, events []*TimelineEvent) *SessionTimeline {
	st := &SessionTimeline{
		ScopeKind: scopeKind,
		ScopeID:   scopeID,
		Events:    events,
	}
	if len(events) == 0 {
		return st
	}

	st.StartTime = events[0].Timestamp
	st.EndTime = events[len(events)-1].Timestamp

	seen := map[int32]bool{}
	groups := map[int32]*LayerGroup{}
	var peak float32
	anomalies := 0

	for _, e := range events {
		if e.BaselineDeviation > peak {
			peak = e.BaselineDeviation
		}
		if e.IsAnomaly {
			anomalies++
		}
		if !seen[e.Layer] {
			seen[e.Layer] = true
			st.Aggregates.LayerActivationSequence = append(st.Aggregates.LayerActivationSequence, e.Layer)
		}
		g, ok := groups[e.Layer]
		if !ok {
			g = &LayerGroup{Layer: e.Layer}
			groups[e.Layer] = g
		}
		g.Count++
		g.Events = append(g.Events, e)
	}

	// Preserve first-seen layer order in ByLayer.
	for _, l := range st.Aggregates.LayerActivationSequence {
		st.ByLayer = append(st.ByLayer, *groups[l])
	}

	st.Aggregates.PeakDeviation = peak
	st.Aggregates.AnomalyCount = anomalies
	st.Aggregates.TotalSignals = len(events)
	st.Aggregates.DurationMS = st.EndTime.Sub(st.StartTime).Milliseconds()
	return st
}
