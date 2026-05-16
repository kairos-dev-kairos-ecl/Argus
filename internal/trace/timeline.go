package trace

import "time"

// TimelineEvent is a single signal rendered as a timeline entry.
type TimelineEvent struct {
	SignalID          string                 `json:"signal_id"`
	Timestamp         time.Time              `json:"timestamp"`
	Layer             int32                  `json:"layer"`
	LayerLabel        string                 `json:"layer_label"`
	Category          string                 `json:"category"`
	Severity          int32                  `json:"severity"`
	BaselineDeviation float32                `json:"baseline_deviation"`
	IsAnomaly         bool                   `json:"is_anomaly"`
	DurationMS        float32                `json:"duration_ms"`
	CtxSummary        map[string]interface{} `json:"ctx_summary,omitempty"`
}

// LayerGroup groups timeline events by layer number.
type LayerGroup struct {
	Layer  int32            `json:"layer"`
	Count  int              `json:"count"`
	Events []*TimelineEvent `json:"events"`
}

// SessionAggregates holds rolled-up statistics for a session or conversation scope.
type SessionAggregates struct {
	PeakDeviation           float32 `json:"peak_deviation"`
	LayerActivationSequence []int32 `json:"layer_activation_sequence"`
	AnomalyCount            int     `json:"anomaly_count"`
	TotalSignals            int     `json:"total_signals"`
	DurationMS              int64   `json:"duration_ms"`
}

// SessionTimeline is the reconstructed signal sequence for a session or conversation scope.
type SessionTimeline struct {
	ScopeKind  string            `json:"scope_kind"` // "session" | "conversation"
	ScopeID    string            `json:"scope_id"`
	StartTime  time.Time         `json:"start_time"`
	EndTime    time.Time         `json:"end_time"`
	Events     []*TimelineEvent  `json:"events"`
	ByLayer    []LayerGroup      `json:"by_layer"`
	Aggregates SessionAggregates `json:"aggregates"`
}
