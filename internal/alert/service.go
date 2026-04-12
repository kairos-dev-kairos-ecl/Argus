package alert

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// AlertService provides CRUD operations for alerts in PostgreSQL.
type AlertService interface {
	Create(ctx context.Context, alert *Alert) (*Alert, error)
	Get(ctx context.Context, alertID uuid.UUID) (*Alert, error)
	List(ctx context.Context, filter *AlertListFilter) ([]*Alert, error)
	Update(ctx context.Context, alertID uuid.UUID, updates *AlertUpdate) error
	UpdateStatus(ctx context.Context, alertID uuid.UUID, newStatus AlertStatus) error
}

// AlertListFilter provides optional filtering criteria for List queries.
type AlertListFilter struct {
	Status     *AlertStatus
	RuleID     *uuid.UUID
	Severity   *Severity
	IncidentID *uuid.UUID
	StartTime  *time.Time
	EndTime    *time.Time
	Limit      int
	Offset     int
}

// AlertUpdate specifies which fields to update (all pointer types to allow partial updates).
type AlertUpdate struct {
	Status       *AlertStatus
	IncidentID   *uuid.UUID
	DedupCount   *int
	AcknowledgedAt *time.Time
	ResolvedAt   *time.Time
	ClosedAt     *time.Time
}

// PostgresAlertService implements AlertService against PostgreSQL.
type PostgresAlertService struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

// NewAlertService creates a new AlertService backed by a PostgreSQL connection pool.
func NewAlertService(pool *pgxpool.Pool, logger *zap.Logger) AlertService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &PostgresAlertService{
		pool:   pool,
		logger: logger,
	}
}

// Create persists a new alert to PostgreSQL and returns the generated alert_id.
// Transaction is atomic; duplicate fingerprints are allowed (dedup check happens in dedup.go).
func (s *PostgresAlertService) Create(ctx context.Context, alert *Alert) (*Alert, error) {
	// Generate alert_id if not provided
	if alert.AlertID == uuid.Nil {
		alert.AlertID = uuid.New()
	}

	// Set timestamps
	now := time.Now()
	if alert.CreatedAt.IsZero() {
		alert.CreatedAt = now
	}
	if alert.UpdatedAt.IsZero() {
		alert.UpdatedAt = now
	}

	// Default status to open if not set
	if alert.Status == "" {
		alert.Status = AlertStatusOpen
	}

	// Default dedup_count to 1 if not set
	if alert.DedupCount == 0 {
		alert.DedupCount = 1
	}

	// Insert with transaction for atomicity
	err := s.pool.QueryRow(ctx,
		`INSERT INTO alerts (
			alert_id, rule_id, signal_ids, severity, confidence,
			fingerprint, dedup_count, status, created_at, updated_at, incident_id, suppressed_by_window_id, created_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
		)
		RETURNING alert_id, created_at, updated_at`,
		alert.AlertID, alert.RuleID, alert.SignalIDs, int(alert.Severity), alert.Confidence,
		alert.Fingerprint, alert.DedupCount, string(alert.Status), alert.CreatedAt, alert.UpdatedAt,
		alert.IncidentID, alert.SuppressedBy, alert.CreatedBy,
	).Scan(&alert.AlertID, &alert.CreatedAt, &alert.UpdatedAt)

	if err != nil {
		s.logger.Error("failed to create alert", zap.Error(err), zap.String("fingerprint", alert.Fingerprint))
		return nil, fmt.Errorf("failed to create alert: %w", err)
	}

	s.logger.Debug("alert created", zap.String("alert_id", alert.AlertID.String()), zap.String("fingerprint", alert.Fingerprint))
	return alert, nil
}

// Get retrieves an alert by ID from PostgreSQL.
func (s *PostgresAlertService) Get(ctx context.Context, alertID uuid.UUID) (*Alert, error) {
	alert := &Alert{}

	err := s.pool.QueryRow(ctx,
		`SELECT
			alert_id, rule_id, signal_ids, severity, confidence,
			fingerprint, dedup_count, status, created_at, acknowledged_at, resolved_at, closed_at,
			incident_id, suppressed_by_window_id, created_by, updated_at
		FROM alerts
		WHERE alert_id = $1`,
		alertID,
	).Scan(
		&alert.AlertID, &alert.RuleID, &alert.SignalIDs, (*int)(&alert.Severity), &alert.Confidence,
		&alert.Fingerprint, &alert.DedupCount, (*string)(&alert.Status), &alert.CreatedAt, &alert.AcknowledgedAt, &alert.ResolvedAt, &alert.ClosedAt,
		&alert.IncidentID, &alert.SuppressedBy, &alert.CreatedBy, &alert.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("alert not found: %s", alertID)
	}
	if err != nil {
		s.logger.Error("failed to get alert", zap.Error(err), zap.String("alert_id", alertID.String()))
		return nil, fmt.Errorf("failed to get alert: %w", err)
	}

	return alert, nil
}

// List returns a filtered list of alerts from PostgreSQL.
// Supports filtering by status, rule_id, severity, incident_id, and time range.
func (s *PostgresAlertService) List(ctx context.Context, filter *AlertListFilter) ([]*Alert, error) {
	if filter == nil {
		filter = &AlertListFilter{Limit: 1000, Offset: 0}
	}

	// Set defaults
	if filter.Limit == 0 {
		filter.Limit = 1000
	}
	if filter.Limit > 10000 {
		filter.Limit = 10000 // Cap to prevent resource exhaustion
	}

	// Build query
	query := `
		SELECT
			alert_id, rule_id, signal_ids, severity, confidence,
			fingerprint, dedup_count, status, created_at, acknowledged_at, resolved_at, closed_at,
			incident_id, suppressed_by_window_id, created_by, updated_at
		FROM alerts
		WHERE 1=1
	`

	var args []interface{}
	argIndex := 1

	if filter.Status != nil {
		query += fmt.Sprintf(` AND status = $%d`, argIndex)
		args = append(args, string(*filter.Status))
		argIndex++
	}

	if filter.RuleID != nil {
		query += fmt.Sprintf(` AND rule_id = $%d`, argIndex)
		args = append(args, *filter.RuleID)
		argIndex++
	}

	if filter.Severity != nil {
		query += fmt.Sprintf(` AND severity = $%d`, argIndex)
		args = append(args, int(*filter.Severity))
		argIndex++
	}

	if filter.IncidentID != nil {
		query += fmt.Sprintf(` AND incident_id = $%d`, argIndex)
		args = append(args, *filter.IncidentID)
		argIndex++
	}

	if filter.StartTime != nil {
		query += fmt.Sprintf(` AND created_at >= $%d`, argIndex)
		args = append(args, *filter.StartTime)
		argIndex++
	}

	if filter.EndTime != nil {
		query += fmt.Sprintf(` AND created_at <= $%d`, argIndex)
		args = append(args, *filter.EndTime)
		argIndex++
	}

	query += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, argIndex, argIndex+1)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		s.logger.Error("failed to list alerts", zap.Error(err))
		return nil, fmt.Errorf("failed to list alerts: %w", err)
	}
	defer rows.Close()

	alerts := []*Alert{}
	for rows.Next() {
		alert := &Alert{}
		err := rows.Scan(
			&alert.AlertID, &alert.RuleID, &alert.SignalIDs, (*int)(&alert.Severity), &alert.Confidence,
			&alert.Fingerprint, &alert.DedupCount, (*string)(&alert.Status), &alert.CreatedAt, &alert.AcknowledgedAt, &alert.ResolvedAt, &alert.ClosedAt,
			&alert.IncidentID, &alert.SuppressedBy, &alert.CreatedBy, &alert.UpdatedAt,
		)
		if err != nil {
			s.logger.Error("failed to scan alert", zap.Error(err))
			return nil, fmt.Errorf("failed to scan alert: %w", err)
		}
		alerts = append(alerts, alert)
	}

	if err = rows.Err(); err != nil {
		s.logger.Error("error iterating alerts", zap.Error(err))
		return nil, fmt.Errorf("error iterating alerts: %w", err)
	}

	return alerts, nil
}

// Update applies partial updates to an alert's mutable fields.
// Only non-nil fields in updates are applied.
func (s *PostgresAlertService) Update(ctx context.Context, alertID uuid.UUID, updates *AlertUpdate) error {
	if updates == nil {
		return nil
	}

	// Build dynamic update query
	query := `UPDATE alerts SET `
	var args []interface{}
	argIndex := 1
	updates_applied := 0

	if updates.Status != nil {
		if updates_applied > 0 {
			query += ", "
		}
		query += fmt.Sprintf(`status = $%d`, argIndex)
		args = append(args, string(*updates.Status))
		argIndex++
		updates_applied++
	}

	if updates.IncidentID != nil {
		if updates_applied > 0 {
			query += ", "
		}
		query += fmt.Sprintf(`incident_id = $%d`, argIndex)
		args = append(args, *updates.IncidentID)
		argIndex++
		updates_applied++
	}

	if updates.DedupCount != nil {
		if updates_applied > 0 {
			query += ", "
		}
		query += fmt.Sprintf(`dedup_count = $%d`, argIndex)
		args = append(args, *updates.DedupCount)
		argIndex++
		updates_applied++
	}

	if updates.AcknowledgedAt != nil {
		if updates_applied > 0 {
			query += ", "
		}
		query += fmt.Sprintf(`acknowledged_at = $%d`, argIndex)
		args = append(args, *updates.AcknowledgedAt)
		argIndex++
		updates_applied++
	}

	if updates.ResolvedAt != nil {
		if updates_applied > 0 {
			query += ", "
		}
		query += fmt.Sprintf(`resolved_at = $%d`, argIndex)
		args = append(args, *updates.ResolvedAt)
		argIndex++
		updates_applied++
	}

	if updates.ClosedAt != nil {
		if updates_applied > 0 {
			query += ", "
		}
		query += fmt.Sprintf(`closed_at = $%d`, argIndex)
		args = append(args, *updates.ClosedAt)
		argIndex++
		updates_applied++
	}

	if updates_applied == 0 {
		return nil // No updates to apply
	}

	// Always update updated_at
	query += fmt.Sprintf(`, updated_at = $%d`, argIndex)
	args = append(args, time.Now())
	argIndex++

	query += fmt.Sprintf(` WHERE alert_id = $%d`, argIndex)
	args = append(args, alertID)

	result, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		s.logger.Error("failed to update alert", zap.Error(err), zap.String("alert_id", alertID.String()))
		return fmt.Errorf("failed to update alert: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("alert not found: %s", alertID)
	}

	s.logger.Debug("alert updated", zap.String("alert_id", alertID.String()))
	return nil
}

// UpdateStatus is a convenience method to update only the status field.
// It validates the transition before updating.
func (s *PostgresAlertService) UpdateStatus(ctx context.Context, alertID uuid.UUID, newStatus AlertStatus) error {
	alert, err := s.Get(ctx, alertID)
	if err != nil {
		return err
	}

	// Validate transition
	if err := ValidateStatusTransition(alert.Status, newStatus); err != nil {
		s.logger.Warn("invalid status transition", zap.String("alert_id", alertID.String()), zap.Error(err))
		return err
	}

	// Set appropriate timestamp based on new status
	now := time.Now()
	updates := &AlertUpdate{Status: &newStatus}

	if newStatus == AlertStatusAcknowledged && alert.AcknowledgedAt == nil {
		updates.AcknowledgedAt = &now
	} else if newStatus == AlertStatusResolved && alert.ResolvedAt == nil {
		updates.ResolvedAt = &now
	} else if newStatus == AlertStatusClosed && alert.ClosedAt == nil {
		updates.ClosedAt = &now
	}

	return s.Update(ctx, alertID, updates)
}
