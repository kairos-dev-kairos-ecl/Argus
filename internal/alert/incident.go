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

// IncidentService provides incident lifecycle management and alert merge operations.
type IncidentService interface {
	CreateFromAlert(ctx context.Context, alert *Alert, title, description string) (*Incident, error)
	Get(ctx context.Context, incidentID uuid.UUID) (*Incident, error)
	List(ctx context.Context, filter *IncidentListFilter) ([]*Incident, error)
	UpdateStatus(ctx context.Context, incidentID uuid.UUID, newStatus IncidentStatus) error
	Merge(ctx context.Context, sourceIncidentID, targetIncidentID uuid.UUID) error
	Escalate(ctx context.Context, incidentID uuid.UUID, escalationTarget string) error
	Update(ctx context.Context, incidentID uuid.UUID, updates *IncidentUpdate) error
}

// IncidentListFilter provides optional filtering for incident List queries.
type IncidentListFilter struct {
	Status      *IncidentStatus
	Severity    *Severity
	AssignedTo  *string
	StartTime   *time.Time
	EndTime     *time.Time
	Limit       int
	Offset      int
}

// IncidentUpdate specifies which fields to update.
type IncidentUpdate struct {
	Status            *IncidentStatus
	Severity          *Severity
	AssignedTo        *string
	EscalationTarget  *string
	Title             *string
	Description       *string
	AcknowledgedAt    *time.Time
	ResolvedAt        *time.Time
	ClosedAt          *time.Time
}

// PostgresIncidentService implements IncidentService against PostgreSQL.
type PostgresIncidentService struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

// NewIncidentService creates a new IncidentService backed by PostgreSQL.
func NewIncidentService(pool *pgxpool.Pool, logger *zap.Logger) IncidentService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &PostgresIncidentService{
		pool:   pool,
		logger: logger,
	}
}

// CreateFromAlert creates a new incident from an alert and links the alert to it.
// The incident is created in "open" status with the alert's severity.
// The alert's incident_id is updated to point to the new incident.
func (s *PostgresIncidentService) CreateFromAlert(ctx context.Context, alert *Alert, title, description string) (*Incident, error) {
	incidentID := uuid.New()
	now := time.Now()

	// Create incident
	incident := &Incident{
		IncidentID: incidentID,
		Status:     IncidentStatusOpen,
		Severity:   alert.Severity,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if title != "" {
		incident.Title = &title
	}

	if description != "" {
		incident.Description = &description
	}

	err := s.pool.QueryRow(ctx,
		`INSERT INTO incidents (
			incident_id, status, severity, created_at, updated_at, title, description
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		)
		RETURNING incident_id, created_at, updated_at`,
		incidentID, string(IncidentStatusOpen), int(alert.Severity), now, now, incident.Title, incident.Description,
	).Scan(&incident.IncidentID, &incident.CreatedAt, &incident.UpdatedAt)

	if err != nil {
		s.logger.Error("failed to create incident", zap.Error(err))
		return nil, fmt.Errorf("failed to create incident: %w", err)
	}

	// Link alert to incident
	err = s.pool.QueryRow(ctx,
		`UPDATE alerts SET incident_id = $1, updated_at = NOW() WHERE alert_id = $2
		RETURNING alert_id`,
		incidentID, alert.AlertID,
	).Scan(&alert.AlertID)

	if err != nil {
		s.logger.Error("failed to link alert to incident", zap.Error(err), zap.String("alert_id", alert.AlertID.String()), zap.String("incident_id", incidentID.String()))
		return nil, fmt.Errorf("failed to link alert to incident: %w", err)
	}

	s.logger.Info("incident created from alert",
		zap.String("incident_id", incidentID.String()),
		zap.String("alert_id", alert.AlertID.String()))

	return incident, nil
}

// Get retrieves an incident by ID.
func (s *PostgresIncidentService) Get(ctx context.Context, incidentID uuid.UUID) (*Incident, error) {
	incident := &Incident{}

	err := s.pool.QueryRow(ctx,
		`SELECT
			incident_id, status, severity, created_at, acknowledged_at, resolved_at, closed_at,
			assigned_to, escalated_at, escalation_target, title, description, updated_at
		FROM incidents
		WHERE incident_id = $1`,
		incidentID,
	).Scan(
		&incident.IncidentID, (*string)(&incident.Status), (*int)(&incident.Severity), &incident.CreatedAt, &incident.AcknowledgedAt, &incident.ResolvedAt, &incident.ClosedAt,
		&incident.AssignedTo, &incident.EscalatedAt, &incident.EscalationTarget, &incident.Title, &incident.Description, &incident.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("incident not found: %s", incidentID)
	}
	if err != nil {
		s.logger.Error("failed to get incident", zap.Error(err), zap.String("incident_id", incidentID.String()))
		return nil, fmt.Errorf("failed to get incident: %w", err)
	}

	return incident, nil
}

// List returns a filtered list of incidents.
func (s *PostgresIncidentService) List(ctx context.Context, filter *IncidentListFilter) ([]*Incident, error) {
	if filter == nil {
		filter = &IncidentListFilter{Limit: 1000, Offset: 0}
	}

	// Set defaults
	if filter.Limit == 0 {
		filter.Limit = 1000
	}
	if filter.Limit > 10000 {
		filter.Limit = 10000
	}

	// Build query
	query := `
		SELECT
			incident_id, status, severity, created_at, acknowledged_at, resolved_at, closed_at,
			assigned_to, escalated_at, escalation_target, title, description, updated_at
		FROM incidents
		WHERE 1=1
	`

	var args []interface{}
	argIndex := 1

	if filter.Status != nil {
		query += fmt.Sprintf(` AND status = $%d`, argIndex)
		args = append(args, string(*filter.Status))
		argIndex++
	}

	if filter.Severity != nil {
		query += fmt.Sprintf(` AND severity = $%d`, argIndex)
		args = append(args, int(*filter.Severity))
		argIndex++
	}

	if filter.AssignedTo != nil {
		query += fmt.Sprintf(` AND assigned_to = $%d`, argIndex)
		args = append(args, *filter.AssignedTo)
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
		s.logger.Error("failed to list incidents", zap.Error(err))
		return nil, fmt.Errorf("failed to list incidents: %w", err)
	}
	defer rows.Close()

	incidents := []*Incident{}
	for rows.Next() {
		incident := &Incident{}
		err := rows.Scan(
			&incident.IncidentID, (*string)(&incident.Status), (*int)(&incident.Severity), &incident.CreatedAt, &incident.AcknowledgedAt, &incident.ResolvedAt, &incident.ClosedAt,
			&incident.AssignedTo, &incident.EscalatedAt, &incident.EscalationTarget, &incident.Title, &incident.Description, &incident.UpdatedAt,
		)
		if err != nil {
			s.logger.Error("failed to scan incident", zap.Error(err))
			return nil, fmt.Errorf("failed to scan incident: %w", err)
		}
		incidents = append(incidents, incident)
	}

	if err = rows.Err(); err != nil {
		s.logger.Error("error iterating incidents", zap.Error(err))
		return nil, fmt.Errorf("error iterating incidents: %w", err)
	}

	return incidents, nil
}

// UpdateStatus transitions an incident to a new status.
// Validates the transition and sets appropriate timestamps.
func (s *PostgresIncidentService) UpdateStatus(ctx context.Context, incidentID uuid.UUID, newStatus IncidentStatus) error {
	incident, err := s.Get(ctx, incidentID)
	if err != nil {
		return err
	}

	// Validate transition
	if err := ValidateIncidentStatusTransition(incident.Status, newStatus); err != nil {
		s.logger.Warn("invalid incident status transition",
			zap.String("incident_id", incidentID.String()),
			zap.Error(err))
		return err
	}

	// Set appropriate timestamp based on new status
	now := time.Now()
	updates := &IncidentUpdate{Status: &newStatus}

	if newStatus == IncidentStatusAcknowledged && incident.AcknowledgedAt == nil {
		updates.AcknowledgedAt = &now
	} else if newStatus == IncidentStatusResolved && incident.ResolvedAt == nil {
		updates.ResolvedAt = &now
	} else if newStatus == IncidentStatusClosed && incident.ClosedAt == nil {
		updates.ClosedAt = &now
	}

	return s.Update(ctx, incidentID, updates)
}

// Merge combines a source incident into a target incident.
// All alerts from the source incident are reassigned to the target.
// The source incident is closed (not deleted).
// Transaction is atomic for consistency.
func (s *PostgresIncidentService) Merge(ctx context.Context, sourceIncidentID, targetIncidentID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		s.logger.Error("failed to begin transaction", zap.Error(err))
		return fmt.Errorf("failed to begin merge transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Verify both incidents exist
	var sourceStatus, targetStatus string
	err = tx.QueryRow(ctx, `SELECT status FROM incidents WHERE incident_id = $1`, sourceIncidentID).Scan(&sourceStatus)
	if err == pgx.ErrNoRows {
		return fmt.Errorf("source incident not found: %s", sourceIncidentID)
	}
	if err != nil {
		s.logger.Error("failed to fetch source incident", zap.Error(err))
		return fmt.Errorf("failed to fetch source incident: %w", err)
	}

	err = tx.QueryRow(ctx, `SELECT status FROM incidents WHERE incident_id = $1`, targetIncidentID).Scan(&targetStatus)
	if err == pgx.ErrNoRows {
		return fmt.Errorf("target incident not found: %s", targetIncidentID)
	}
	if err != nil {
		s.logger.Error("failed to fetch target incident", zap.Error(err))
		return fmt.Errorf("failed to fetch target incident: %w", err)
	}

	// Prevent merging into a closed incident
	if targetStatus == string(IncidentStatusClosed) {
		return fmt.Errorf("cannot merge into a closed incident")
	}

	// Reassign all alerts from source to target
	result, err := tx.Exec(ctx,
		`UPDATE alerts SET incident_id = $1, updated_at = NOW() WHERE incident_id = $2`,
		targetIncidentID, sourceIncidentID,
	)
	if err != nil {
		s.logger.Error("failed to reassign alerts", zap.Error(err))
		return fmt.Errorf("failed to reassign alerts: %w", err)
	}

	alertsReassigned := result.RowsAffected()

	// Close source incident
	now := time.Now()
	_, err = tx.Exec(ctx,
		`UPDATE incidents SET status = $1, closed_at = $2, updated_at = $3 WHERE incident_id = $4`,
		string(IncidentStatusClosed), now, now, sourceIncidentID,
	)
	if err != nil {
		s.logger.Error("failed to close source incident", zap.Error(err))
		return fmt.Errorf("failed to close source incident: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		s.logger.Error("failed to commit merge transaction", zap.Error(err))
		return fmt.Errorf("failed to commit merge transaction: %w", err)
	}

	s.logger.Info("incidents merged",
		zap.String("source_incident_id", sourceIncidentID.String()),
		zap.String("target_incident_id", targetIncidentID.String()),
		zap.Int64("alerts_reassigned", alertsReassigned))

	return nil
}

// Escalate sets an escalation target for an incident and records the escalation timestamp.
func (s *PostgresIncidentService) Escalate(ctx context.Context, incidentID uuid.UUID, escalationTarget string) error {
	now := time.Now()

	result, err := s.pool.Exec(ctx,
		`UPDATE incidents SET escalation_target = $1, escalated_at = $2, updated_at = $3 WHERE incident_id = $4`,
		escalationTarget, now, now, incidentID,
	)

	if err != nil {
		s.logger.Error("failed to escalate incident", zap.Error(err), zap.String("incident_id", incidentID.String()))
		return fmt.Errorf("failed to escalate incident: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("incident not found: %s", incidentID)
	}

	s.logger.Info("incident escalated",
		zap.String("incident_id", incidentID.String()),
		zap.String("escalation_target", escalationTarget))

	return nil
}

// Update applies partial updates to an incident's fields.
func (s *PostgresIncidentService) Update(ctx context.Context, incidentID uuid.UUID, updates *IncidentUpdate) error {
	if updates == nil {
		return nil
	}

	// Build dynamic update query
	query := `UPDATE incidents SET `
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

	if updates.Severity != nil {
		if updates_applied > 0 {
			query += ", "
		}
		query += fmt.Sprintf(`severity = $%d`, argIndex)
		args = append(args, int(*updates.Severity))
		argIndex++
		updates_applied++
	}

	if updates.AssignedTo != nil {
		if updates_applied > 0 {
			query += ", "
		}
		query += fmt.Sprintf(`assigned_to = $%d`, argIndex)
		args = append(args, *updates.AssignedTo)
		argIndex++
		updates_applied++
	}

	if updates.EscalationTarget != nil {
		if updates_applied > 0 {
			query += ", "
		}
		query += fmt.Sprintf(`escalation_target = $%d`, argIndex)
		args = append(args, *updates.EscalationTarget)
		argIndex++
		updates_applied++
	}

	if updates.Title != nil {
		if updates_applied > 0 {
			query += ", "
		}
		query += fmt.Sprintf(`title = $%d`, argIndex)
		args = append(args, *updates.Title)
		argIndex++
		updates_applied++
	}

	if updates.Description != nil {
		if updates_applied > 0 {
			query += ", "
		}
		query += fmt.Sprintf(`description = $%d`, argIndex)
		args = append(args, *updates.Description)
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
		return nil
	}

	// Always update updated_at
	query += fmt.Sprintf(`, updated_at = $%d`, argIndex)
	args = append(args, time.Now())
	argIndex++

	query += fmt.Sprintf(` WHERE incident_id = $%d`, argIndex)
	args = append(args, incidentID)

	result, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		s.logger.Error("failed to update incident", zap.Error(err), zap.String("incident_id", incidentID.String()))
		return fmt.Errorf("failed to update incident: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("incident not found: %s", incidentID)
	}

	s.logger.Debug("incident updated", zap.String("incident_id", incidentID.String()))
	return nil
}
