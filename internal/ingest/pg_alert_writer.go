package ingest

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/argusxdr/argus/internal/detection/engine"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// PgAlertWriter writes detection match results to PostgreSQL.
type PgAlertWriter struct {
	pool *pgxpool.Pool
	log  *zap.Logger
}

// NewPgAlertWriter creates an AlertWriter backed by PostgreSQL.
func NewPgAlertWriter(pool *pgxpool.Pool, log *zap.Logger) *PgAlertWriter {
	if log == nil {
		log = zap.NewNop()
	}
	return &PgAlertWriter{pool: pool, log: log}
}

// WriteAlert inserts an alert row. If no pool is configured, this is a no-op.
func (w *PgAlertWriter) WriteAlert(ctx context.Context, m engine.MatchResult) error {
	if w.pool == nil {
		return nil
	}

	appID := ""
	traceID := ""
	signalID := ""
	layer := 0
	category := ""
	if m.Signal != nil {
		if m.Signal.Source != nil {
			appID = m.Signal.Source.AppId
		}
		traceID = m.Signal.TraceId
		signalID = m.Signal.SignalId
		layer = int(m.Signal.Layer)
		category = m.Signal.Category
	}

	fingerprint := computeAlertFingerprint(m.Rule.ID, signalID)
	now := time.Now()
	_, err := w.pool.Exec(ctx, `
		INSERT INTO alerts (
			id, app_id, fingerprint, severity, layer, category,
			title, description, signal_ids, trace_id, status,
			signal_count, first_seen_at, last_seen_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, 'open',
			1, $11, $11
		) ON CONFLICT (fingerprint) DO UPDATE SET
			signal_count = alerts.signal_count + 1,
			last_seen_at = $11
	`,
		uuid.NewString(),
		appID,
		fingerprint,
		m.Rule.Severity,
		layer,
		category,
		m.Rule.Action.Title,
		m.Rule.Action.Description,
		[]string{signalID},
		traceID,
		now,
	)
	if err != nil {
		w.log.Warn("failed to write alert", zap.String("rule_id", m.Rule.ID), zap.Error(err))
		return fmt.Errorf("write alert: %w", err)
	}
	return nil
}

func computeAlertFingerprint(ruleID, signalID string) string {
	h := sha256.Sum256([]byte(ruleID + ":" + signalID))
	return fmt.Sprintf("%x", h[:])
}
