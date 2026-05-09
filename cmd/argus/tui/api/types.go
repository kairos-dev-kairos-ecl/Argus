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

// SignalSource is the nested source/app metadata in an ArgusSignal.
// Maps to the proto Source message serialized by encoding/json.
type SignalSource struct {
	AppID       string `json:"app_id"`
	AppVersion  string `json:"app_version"`
	SDKVersion  string `json:"sdk_version"`
	Environment string `json:"environment"`
	InstanceID  string `json:"instance_id"`
}

// SignalEnrichment contains computed enrichment fields from the processing pipeline.
// Maps to the proto Enrichment message; fields are optional (*float32 may be nil).
type SignalEnrichment struct {
	BaselineDeviation *float32 `json:"baseline_deviation,omitempty"`
	RiskScore         *float32 `json:"risk_score,omitempty"`
}

// Signal is a minimal DTO for a signal record from the Argus backend.
// The backend serializes *v1.ArgusSignal with encoding/json, so:
//   - layer and severity are integers (proto enum)
//   - timestamp is {"seconds":N,"nanos":N} (timestamppb.Timestamp)
//   - source is a nested object (no top-level app_id)
//   - enrichment is a nullable nested object
type Signal struct {
	ID         string            `json:"signal_id"`
	TraceID    string            `json:"trace_id"`
	SpanID     string            `json:"span_id"`
	Source     SignalSource      `json:"source"`
	Layer      int               `json:"layer"`
	Category   string            `json:"category"`
	Severity   int               `json:"severity"`
	Timestamp  ProtoTime         `json:"timestamp"`
	Enrichment *SignalEnrichment `json:"enrichment,omitempty"`
}

// AnomalyScore returns the baseline deviation enrichment value, or 0 if absent.
func (s Signal) AnomalyScore() float64 {
	if s.Enrichment != nil && s.Enrichment.BaselineDeviation != nil {
		return float64(*s.Enrichment.BaselineDeviation)
	}
	return 0
}

// Alert represents a fired detection rule alert.
// Matches the alertRow struct serialized by handleListAlerts.
type Alert struct {
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

// Rule is a detection rule definition.
// Matches the RuleView struct serialized by handleListRules.
// The Config field is decoded from the backend's "config" JSON object
// and stored as a string so the editor workflow can read/write it.
type Rule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Tier        int    `json:"tier"`
	Enabled     bool   `json:"enabled"`
	// YAML holds the raw config JSON from the backend; named YAML for historical
	// compatibility with the editor workflow (launchEditor writes it to a temp file).
	YAML      string `json:"-"` // populated by UnmarshalJSON from "config"
	CreatedAt string `json:"created_at"` // RFC3339 string
	UpdatedAt string `json:"updated_at"` // RFC3339 string
}

// ruleJSON is the intermediate struct for decoding the backend RuleView JSON.
// The backend's "config" field is a json.RawMessage (embedded JSON object).
type ruleJSON struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Tier        int             `json:"tier"`
	Enabled     bool            `json:"enabled"`
	Config      json.RawMessage `json:"config"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
}

// UnmarshalJSON decodes a RuleView JSON object, mapping the "config" object
// to the YAML string field (stored as pretty-printed JSON for editor display).
func (r *Rule) UnmarshalJSON(data []byte) error {
	var rj ruleJSON
	if err := json.Unmarshal(data, &rj); err != nil {
		return err
	}
	r.ID = rj.ID
	r.Name = rj.Name
	r.Description = rj.Description
	r.Tier = rj.Tier
	r.Enabled = rj.Enabled
	r.CreatedAt = rj.CreatedAt
	r.UpdatedAt = rj.UpdatedAt
	if len(rj.Config) > 0 {
		// Pretty-print the config JSON so the editor shows readable content.
		if pretty, err := json.MarshalIndent(rj.Config, "", "  "); err == nil {
			r.YAML = string(pretty)
		} else {
			r.YAML = string(rj.Config)
		}
	}
	return nil
}

// AuditEntry is a row in the system audit log.
// Matches the auditEntryDTO struct serialized by handleListAuditLog.
type AuditEntry struct {
	ID           string                 `json:"id"`
	Timestamp    string                 `json:"timestamp"`  // RFC3339 string
	Action       string                 `json:"action"`
	ActorID      *string                `json:"actor_id"`
	ActorEmail   *string                `json:"actor_email"`
	ResourceType string                 `json:"resource_type"`
	ResourceID   string                 `json:"resource_id"`
	IPAddress    string                 `json:"ip_address"`
	Hash         string                 `json:"hash"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// ActorEmailStr returns the actor email string, or empty string if nil.
func (e AuditEntry) ActorEmailStr() string {
	if e.ActorEmail != nil {
		return *e.ActorEmail
	}
	return ""
}

// User represents an Argus user account.
// Matches the userResponse struct serialized by handleListUsers.
type User struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Status      string `json:"status"`
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
