// Package api provides HTTP and WebSocket clients for the Argus TUI.
package api

import (
	"encoding/json"
	"time"
)

// ProtoTime handles both timestamp shapes that the Argus backend may emit:
//   - RFC3339 string:              "2024-01-15T10:42:13Z"          (protojson)
//   - Proto object:                {"seconds":1705315333,"nanos":0} (encoding/json on *timestamppb.Timestamp)
//
// Embeds time.Time so all time.Time methods (Format, IsZero, etc.) work
// transparently on callers — no screen code changes needed.
type ProtoTime struct{ time.Time }

func (t *ProtoTime) UnmarshalJSON(data []byte) error {
	// RFC3339 string path
	if len(data) > 0 && data[0] == '"' {
		return t.Time.UnmarshalJSON(data)
	}
	// Proto object path: {"seconds":N,"nanos":N}
	var p struct {
		Seconds int64 `json:"seconds"`
		Nanos   int32 `json:"nanos"`
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	t.Time = time.Unix(p.Seconds, int64(p.Nanos)).UTC()
	return nil
}

// User represents an Argus user account.
type User struct {
	ID         string    `json:"id"`
	Email      string    `json:"email"`
	Role       string    `json:"role"`
	CreatedAt  ProtoTime `json:"created_at"`
	MFAEnabled bool      `json:"mfa_enabled"`
}

// Signal is a minimal DTO for a signal record from the Argus backend.
// Full field set comes from the proto schema; only essential display fields are
// included here to avoid over-fetching.
type Signal struct {
	ID           string    `json:"signal_id"`
	TraceID      string    `json:"trace_id"`
	AppID        string    `json:"app_id"`
	Layer        int       `json:"layer"`
	Category     string    `json:"category"`
	Severity     string    `json:"severity"`
	Message      string    `json:"message"`
	Timestamp    ProtoTime `json:"timestamp"`
	AnomalyScore float64   `json:"anomaly_score"`
}

// Alert represents a fired detection rule alert.
type Alert struct {
	ID          string    `json:"id"`
	RuleID      string    `json:"rule_id"`
	RuleName    string    `json:"rule_name"`
	Severity    string    `json:"severity"`
	Message     string    `json:"message"`
	Status      string    `json:"status"`
	TraceID     string    `json:"trace_id"`
	CreatedAt   time.Time `json:"created_at"`
	AckedAt     *time.Time `json:"acked_at,omitempty"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
}

// Rule is a detection rule definition.
type Rule struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Severity    string    `json:"severity"`
	Enabled     bool      `json:"enabled"`
	YAML        string    `json:"yaml"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AuditEntry is a row in the system audit log.
type AuditEntry struct {
	ID        string    `json:"id"`
	UserEmail string    `json:"user_email"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	IPAddress string    `json:"ip_address"`
	CreatedAt time.Time `json:"created_at"`
}

// Span is one entry in a TraceResponse.Spans slice.
// Mirrors internal/ingest.SpanView — layer is the full proto enum string
// (e.g. "L1_HARDWARE"), not an integer.
type Span struct {
	SignalID       string  `json:"signal_id"`
	Layer          string  `json:"layer"`
	StartTime      string  `json:"start_time"`
	DurationMs     float64 `json:"duration_ms"`
	ParentSignalID string  `json:"parent_signal_id,omitempty"`
	Status         string  `json:"status"`
	Message        string  `json:"message"`
}

// TraceResponse is the JSON envelope from GET /api/v1/traces/{traceId}.
type TraceResponse struct {
	TraceID    string `json:"trace_id"`
	Spans      []Span `json:"spans"`
	DurationMs int64  `json:"duration_ms"`
}

// SpanLayerInt maps the proto enum layer string to the integer (1–10) used
// by the TUI render layer. Unknown values map to 0.
var SpanLayerInt = map[string]int{
	"L1_HARDWARE":        1,
	"L2_MODEL_WEIGHTS":   2,
	"L3_TOKENIZER":       3,
	"L4_TRANSFORMER":     4,
	"L5_OUTPUT_DECODING": 5,
	"L6_SAFETY":          6,
	"L7_RAG_RETRIEVAL":   7,
	"L8_AGENTS":          8,
	"L9_API_GATEWAY":     9,
	"L10_APPLICATION":    10,
}

// WSMsg is the envelope for WebSocket messages from /api/v1/signals/stream.
type WSMsg struct {
	Type    string `json:"type"`
	Payload Signal `json:"payload"`
}

// APIError is a typed error returned when the backend responds with a non-2xx
// status code.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return e.Message
}
