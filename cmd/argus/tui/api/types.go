// Package api provides HTTP and WebSocket clients for the Argus TUI.
package api

import "time"

// User represents an Argus user account.
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	MFAEnabled bool     `json:"mfa_enabled"`
}

// Signal is a minimal DTO for a signal record from the Argus backend.
// Full field set comes from the proto schema; only essential display fields are
// included here to avoid over-fetching.
type Signal struct {
	ID         string    `json:"signal_id"`
	TraceID    string    `json:"trace_id"`
	AppID      string    `json:"app_id"`
	Layer      int       `json:"layer"`
	Category   string    `json:"category"`
	Severity   string    `json:"severity"`
	Message    string    `json:"message"`
	Timestamp  time.Time `json:"timestamp"`
	AnomalyScore float64 `json:"anomaly_score"`
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
