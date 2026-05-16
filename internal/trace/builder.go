package trace

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// ErrTraceNotFound is returned when no signals match the requested trace_id.
var ErrTraceNotFound = errors.New("trace not found")

// RunReconstructor rebuilds a typed run DAG from signals stored in ClickHouse.
// It is a query-time component — not a pipeline stage — and is safe for concurrent use.
type RunReconstructor struct {
	ch driver.Conn
}

// NewRunReconstructor creates a RunReconstructor backed by the given ClickHouse connection.
func NewRunReconstructor(ch driver.Conn) *RunReconstructor {
	return &RunReconstructor{ch: ch}
}

// BuildRun fetches all signals for traceID and reconstructs the causal DAG.
// Returns ErrTraceNotFound if no signals exist for the trace.
func (r *RunReconstructor) BuildRun(ctx context.Context, traceID string) (*RunGraph, error) {
	if traceID == "" {
		return nil, fmt.Errorf("traceID required")
	}
	rows, err := r.ch.Query(ctx, SelectRunSignalsSQL, traceID)
	if err != nil {
		return nil, fmt.Errorf("query run signals: %w", err)
	}
	defer rows.Close()

	var nodes []*RunNode
	for rows.Next() {
		n, err := scanRunNode(rows)
		if err != nil {
			return nil, fmt.Errorf("scan run node: %w", err)
		}
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	if len(nodes) == 0 {
		return nil, ErrTraceNotFound
	}
	return assembleGraph(traceID, nodes), nil
}

// scanRunNode scans one row from SelectRunSignalsSQL into a RunNode.
// Nullable columns are scanned via sql.Null* types; CtxSummary is populated
// with only the non-null context fields.
func scanRunNode(rows driver.Rows) (*RunNode, error) {
	var (
		signalID    string
		spanID      string
		parentSpanID sql.NullString
		layer       uint8
		category    string
		severity    uint8
		timestamp   time.Time
		durationMS  sql.NullFloat64
		baselineDev sql.NullFloat64

		// L3 Tokenizer
		l3InputTokens  sql.NullInt64
		l3OutputTokens sql.NullInt64
		l3TokenEntropy sql.NullFloat64

		// L4 Transformer
		l4KVCacheHitRate sql.NullFloat64
		l4DecodeTimeMS   sql.NullFloat64

		// L5 Output
		l5Temperature  sql.NullFloat64
		l5MeanLogprob  sql.NullFloat64
		l5EntropyMean  sql.NullFloat64
		l5FinishReason sql.NullString

		// L6 Safety
		l6SafetyScore          sql.NullFloat64
		l6JailbreakScore       sql.NullFloat64
		l6PromptInjectionScore sql.NullFloat64

		// L7 RAG
		l7ResultsCount    sql.NullInt64
		l7LatencySearchMS sql.NullFloat64
		l7QueryCacheHit   sql.NullInt64 // Nullable(UInt8)

		// L8 Agents
		l8ToolName     sql.NullString
		l8ToolLatencyMS sql.NullFloat64
		l8StepNumber   sql.NullInt64

		// L9 API Gateway
		l9Method     sql.NullString
		l9Path       sql.NullString
		l9StatusCode sql.NullInt64
		l9LatencyMS  sql.NullFloat64

		// L10 Application
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

	n := &RunNode{
		SignalID:  signalID,
		SpanID:    spanID,
		Layer:     int32(layer),
		Category:  category,
		Severity:  int32(severity),
		Timestamp: timestamp,
	}
	if parentSpanID.Valid {
		n.ParentSpanID = parentSpanID.String
	}
	if durationMS.Valid {
		n.DurationMS = float32(durationMS.Float64)
	}
	if baselineDev.Valid {
		n.BaselineDeviation = float32(baselineDev.Float64)
	}

	// Build CtxSummary with only non-null context fields.
	ctx := make(map[string]interface{})
	if l3InputTokens.Valid {
		ctx["l3_input_tokens"] = l3InputTokens.Int64
	}
	if l3OutputTokens.Valid {
		ctx["l3_output_tokens"] = l3OutputTokens.Int64
	}
	if l3TokenEntropy.Valid {
		ctx["l3_token_entropy"] = l3TokenEntropy.Float64
	}
	if l4KVCacheHitRate.Valid {
		ctx["l4_kv_cache_hit_rate"] = l4KVCacheHitRate.Float64
	}
	if l4DecodeTimeMS.Valid {
		ctx["l4_decode_time_ms"] = l4DecodeTimeMS.Float64
	}
	if l5Temperature.Valid {
		ctx["l5_temperature"] = l5Temperature.Float64
	}
	if l5MeanLogprob.Valid {
		ctx["l5_mean_logprob"] = l5MeanLogprob.Float64
	}
	if l5EntropyMean.Valid {
		ctx["l5_entropy_mean"] = l5EntropyMean.Float64
	}
	if l5FinishReason.Valid {
		ctx["l5_finish_reason"] = l5FinishReason.String
	}
	if l6SafetyScore.Valid {
		ctx["l6_safety_score"] = l6SafetyScore.Float64
	}
	if l6JailbreakScore.Valid {
		ctx["l6_jailbreak_score"] = l6JailbreakScore.Float64
	}
	if l6PromptInjectionScore.Valid {
		ctx["l6_prompt_injection_score"] = l6PromptInjectionScore.Float64
	}
	if l7ResultsCount.Valid {
		ctx["l7_results_count"] = l7ResultsCount.Int64
	}
	if l7LatencySearchMS.Valid {
		ctx["l7_latency_search_ms"] = l7LatencySearchMS.Float64
	}
	if l7QueryCacheHit.Valid {
		ctx["l7_query_cache_hit"] = l7QueryCacheHit.Int64 != 0
	}
	if l8ToolName.Valid {
		ctx["l8_tool_name"] = l8ToolName.String
	}
	if l8ToolLatencyMS.Valid {
		ctx["l8_tool_latency_ms"] = l8ToolLatencyMS.Float64
	}
	if l8StepNumber.Valid {
		ctx["l8_step_number"] = l8StepNumber.Int64
	}
	if l9Method.Valid {
		ctx["l9_method"] = l9Method.String
	}
	if l9Path.Valid {
		ctx["l9_path"] = l9Path.String
	}
	if l9StatusCode.Valid {
		ctx["l9_status_code"] = l9StatusCode.Int64
	}
	if l9LatencyMS.Valid {
		ctx["l9_latency_ms"] = l9LatencyMS.Float64
	}
	if l10UserID.Valid {
		ctx["l10_user_id"] = l10UserID.String
	}
	if l10SessionID.Valid {
		ctx["l10_session_id"] = l10SessionID.String
	}
	if l10ConversationID.Valid {
		ctx["l10_conversation_id"] = l10ConversationID.String
	}
	if l10EventType.Valid {
		ctx["l10_event_type"] = l10EventType.String
	}
	if len(ctx) > 0 {
		n.CtxSummary = ctx
	}
	return n, nil
}

// assembleGraph walks parent_span_id relationships to build a RunGraph from nodes.
// Orphan detection: if parent_span_id is set but the parent is not in the node set,
// the node is flagged IsOrphan=true and a synthetic temporal edge attaches it to the root.
func assembleGraph(traceID string, nodes []*RunNode) *RunGraph {
	bySpan := make(map[string]*RunNode, len(nodes))
	for _, n := range nodes {
		bySpan[n.SpanID] = n
	}

	var edges []RunEdge
	var orphans []*RunNode
	var roots []*RunNode
	layerSet := map[int32]struct{}{}
	var peak float32
	var start, end time.Time

	for i, n := range nodes {
		if i == 0 || n.Timestamp.Before(start) {
			start = n.Timestamp
		}
		if n.Timestamp.After(end) {
			end = n.Timestamp
		}
		if n.BaselineDeviation > peak {
			peak = n.BaselineDeviation
		}
		n.IsAnomaly = n.BaselineDeviation > AnomalyDeviationThreshold
		layerSet[n.Layer] = struct{}{}

		if n.ParentSpanID == "" {
			roots = append(roots, n)
			continue
		}
		if _, ok := bySpan[n.ParentSpanID]; ok {
			edges = append(edges, RunEdge{
				FromSpanID: n.ParentSpanID,
				ToSpanID:   n.SpanID,
				Type:       EdgeParentChild,
			})
		} else {
			orphans = append(orphans, n)
		}
	}

	attachOrphans(roots, orphans, &edges)

	layers := make([]int32, 0, len(layerSet))
	for l := range layerSet {
		layers = append(layers, l)
	}
	sort.Slice(layers, func(i, j int) bool { return layers[i] < layers[j] })

	return &RunGraph{
		Meta: RunMeta{
			TraceID:       traceID,
			LayersPresent: layers,
			StartTime:     start,
			EndTime:       end,
			PeakDeviation: peak,
			SignalCount:   len(nodes),
			OrphanCount:   len(orphans),
		},
		Nodes: nodes,
		Edges: edges,
	}
}
