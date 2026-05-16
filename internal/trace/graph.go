package trace

import "time"

// AnomalyDeviationThreshold is the z-score threshold above which a signal is considered anomalous.
const AnomalyDeviationThreshold float32 = 2.0

// EdgeType describes the relationship between two nodes in a run graph.
type EdgeType string

const (
	// EdgeParentChild is a direct parent_span_id -> span_id relationship.
	EdgeParentChild EdgeType = "parent_child"
	// EdgeTemporal is a synthetic edge added when an orphan is attached to the graph.
	EdgeTemporal EdgeType = "temporal"
)

// RunNode represents a single signal in a run DAG.
type RunNode struct {
	SignalID          string                 `json:"signal_id"`
	SpanID            string                 `json:"span_id"`
	ParentSpanID      string                 `json:"parent_span_id,omitempty"`
	Layer             int32                  `json:"layer"`
	Category          string                 `json:"category"`
	Severity          int32                  `json:"severity"`
	Timestamp         time.Time              `json:"timestamp"`
	DurationMS        float32                `json:"duration_ms"`
	BaselineDeviation float32                `json:"baseline_deviation"`
	IsAnomaly         bool                   `json:"is_anomaly"`
	IsOrphan          bool                   `json:"is_orphan"`
	CtxSummary        map[string]interface{} `json:"ctx_summary,omitempty"`
}

// RunEdge represents a directed edge between two nodes in a run DAG.
type RunEdge struct {
	FromSpanID string   `json:"from_span_id"`
	ToSpanID   string   `json:"to_span_id"`
	Type       EdgeType `json:"type"`
}

// RunMeta contains aggregate metadata about an entire run.
type RunMeta struct {
	TraceID       string    `json:"trace_id"`
	LayersPresent []int32   `json:"layers_present"`
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time"`
	PeakDeviation float32   `json:"peak_deviation"`
	SignalCount   int       `json:"signal_count"`
	OrphanCount   int       `json:"orphan_count"`
}

// RunGraph is the reconstructed execution tree for a single trace_id.
type RunGraph struct {
	Meta  RunMeta    `json:"meta"`
	Nodes []*RunNode `json:"nodes"`
	Edges []RunEdge  `json:"edges"`
}
