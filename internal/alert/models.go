package alert

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

// AlertStatus represents the lifecycle state of an alert.
// Valid transitions: open -> acknowledged -> resolved -> closed
type AlertStatus string

const (
	AlertStatusOpen         AlertStatus = "open"
	AlertStatusAcknowledged AlertStatus = "acknowledged"
	AlertStatusResolved     AlertStatus = "resolved"
	AlertStatusClosed       AlertStatus = "closed"
)

// Severity represents alert severity on a 1-5 scale.
// 1 = Low, 2 = Medium, 3 = High, 4 = Critical, 5 = Blocker
type Severity int

// Alert represents a fired detection rule match persisted to PostgreSQL.
// Alerts are fingerprinted for deduplication and linked to incidents for lifecycle tracking.
type Alert struct {
	AlertID         uuid.UUID
	RuleID          uuid.UUID
	SignalIDs       []uuid.UUID
	Severity        Severity
	Confidence      float64
	Fingerprint     string // SHA256 hex of concatenated (rule_id + sorted signal_ids)
	DedupCount      int    // Number of duplicate fingerprints suppressed during dedup window
	Status          AlertStatus
	CreatedAt       time.Time
	AcknowledgedAt  *time.Time
	ResolvedAt      *time.Time
	ClosedAt        *time.Time
	IncidentID      *uuid.UUID // FK to incidents table (nullable until incident is created)
	SuppressedBy    *uuid.UUID // FK to suppression_rules (if suppressed)
	CreatedBy       *string    // User who created the alert (optional)
	UpdatedAt       time.Time
}

// ComputeFingerprint generates a deterministic SHA256 fingerprint for deduplication.
// It hashes the concatenation of rule_id + sorted signal_ids.
// Same inputs always produce the same fingerprint (deterministic).
func ComputeFingerprint(ruleID string, signalIDs []uuid.UUID) string {
	// Sort signal IDs to ensure deterministic ordering
	sortedIDs := make([]uuid.UUID, len(signalIDs))
	copy(sortedIDs, signalIDs)
	sort.Slice(sortedIDs, func(i, j int) bool {
		return sortedIDs[i].String() < sortedIDs[j].String()
	})

	// Concatenate rule_id + sorted signal_ids
	data := ruleID
	for _, id := range sortedIDs {
		data += id.String()
	}

	// Hash with SHA256 and return hex string
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash[:])
}

// IncidentStatus represents the lifecycle state of an incident.
// Valid transitions: open -> acknowledged -> resolved -> closed
type IncidentStatus string

const (
	IncidentStatusOpen         IncidentStatus = "open"
	IncidentStatusAcknowledged IncidentStatus = "acknowledged"
	IncidentStatusResolved     IncidentStatus = "resolved"
	IncidentStatusClosed       IncidentStatus = "closed"
)

// Incident represents an incident tracking multiple related alerts.
// Incidents have their own lifecycle independent of individual alerts.
// Multiple alerts can be merged into a single incident.
type Incident struct {
	IncidentID        uuid.UUID
	Status            IncidentStatus
	Severity          Severity
	CreatedAt         time.Time
	AcknowledgedAt    *time.Time
	ResolvedAt        *time.Time
	ClosedAt          *time.Time
	AssignedTo        *string   // User assignment (optional)
	EscalatedAt       *time.Time
	EscalationTarget  *string   // Escalation target (user, team, external system)
	Title             *string   // Incident title
	Description       *string   // Incident description
	UpdatedAt         time.Time
}

// ValidateStatusTransition checks if a status transition is valid.
// Only forward transitions are allowed: open -> acknowledged -> resolved -> closed
// No backward transitions are permitted.
func ValidateStatusTransition(from AlertStatus, to AlertStatus) error {
	validTransitions := map[AlertStatus]map[AlertStatus]bool{
		AlertStatusOpen: {
			AlertStatusAcknowledged: true,
		},
		AlertStatusAcknowledged: {
			AlertStatusResolved: true,
		},
		AlertStatusResolved: {
			AlertStatusClosed: true,
		},
		AlertStatusClosed: {
			// No transitions from closed
		},
	}

	if validTransitions[from][to] {
		return nil
	}
	return fmt.Errorf("invalid alert status transition: %s -> %s", from, to)
}

// ValidateIncidentStatusTransition checks if an incident status transition is valid.
// Only forward transitions are allowed: open -> acknowledged -> resolved -> closed
// No backward transitions are permitted.
func ValidateIncidentStatusTransition(from IncidentStatus, to IncidentStatus) error {
	validTransitions := map[IncidentStatus]map[IncidentStatus]bool{
		IncidentStatusOpen: {
			IncidentStatusAcknowledged: true,
		},
		IncidentStatusAcknowledged: {
			IncidentStatusResolved: true,
		},
		IncidentStatusResolved: {
			IncidentStatusClosed: true,
		},
		IncidentStatusClosed: {
			// No transitions from closed
		},
	}

	if validTransitions[from][to] {
		return nil
	}
	return fmt.Errorf("invalid incident status transition: %s -> %s", from, to)
}
