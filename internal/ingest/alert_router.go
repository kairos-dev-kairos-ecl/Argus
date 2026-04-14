package ingest

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strconv"
	"time"

	"github.com/argusxdr/argus/internal/detection/engine"
	"github.com/argusxdr/argus/internal/notify"
	"github.com/argusxdr/argus/internal/pipeline"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const dedupWindow = 15 * time.Minute

var _ pipeline.AlertWriter = (*AlertRouter)(nil)

type routingEvaluator interface {
	Evaluate(ctx *notify.EvaluationContext) []string
}

type dispatchSink interface {
	Dispatch(job *notify.DispatchJob) error
}

// AlertRouter is the post-detection pipeline owner: dedup -> persist -> route -> dispatch.
type AlertRouter struct {
	pool       *pgxpool.Pool
	redis      *redis.Client
	routing    routingEvaluator
	dispatcher dispatchSink
	log        *zap.Logger
}

// NewAlertRouter creates a new alert router.
func NewAlertRouter(pool *pgxpool.Pool, redisClient *redis.Client, routing routingEvaluator, dispatcher dispatchSink, log *zap.Logger) *AlertRouter {
	if log == nil {
		log = zap.NewNop()
	}
	return &AlertRouter{
		pool:       pool,
		redis:      redisClient,
		routing:    routing,
		dispatcher: dispatcher,
		log:        log,
	}
}

// WriteAlert implements pipeline.AlertWriter.
func (ar *AlertRouter) WriteAlert(ctx context.Context, m engine.MatchResult) error {
	if m.Signal == nil {
		return nil
	}

	appID := ""
	if m.Signal.Source != nil {
		appID = m.Signal.Source.AppId
	}
	traceID := m.Signal.TraceId
	signalID := m.Signal.SignalId
	layer := int(m.Signal.Layer)
	fingerprint := computeRouterFingerprint(m.Rule.ID, appID)

	alertID := uuid.New()
	isNew := true

	if ar.redis != nil {
		ok, err := ar.redis.SetNX(ctx, "dedup:"+fingerprint, alertID.String(), dedupWindow).Result()
		if err != nil {
			ar.log.Warn("dedup redis failed, continuing without dedup", zap.Error(err))
		} else {
			isNew = ok
		}
	}

	if ar.pool != nil {
		var err error
		alertID, err = ar.upsertAlert(ctx, m, alertID, fingerprint, signalID, appID, traceID, layer, isNew)
		if err != nil {
			return err
		}
	}

	if !isNew {
		return nil
	}

	if ar.routing == nil || ar.dispatcher == nil {
		return nil
	}

	targets := ar.routing.Evaluate(&notify.EvaluationContext{
		Severity: m.Rule.Severity,
		AppID:    appID,
		Layer:    layer,
	})
	if len(targets) == 0 {
		return nil
	}

	req := &notify.NotificationRequest{
		ID:       uuid.NewString(),
		AlertID:  alertID,
		Severity: m.Rule.Severity,
		Title:    m.Rule.Action.Title,
		Message:  m.Rule.Action.Description,
		Metadata: map[string]string{
			"app_id":    appID,
			"trace_id":  traceID,
			"signal_id": signalID,
			"layer":     strconv.Itoa(layer),
			"category":  m.Signal.Category,
			"rule_id":   m.Rule.ID,
		},
	}
	if parsedRuleID, err := uuid.Parse(m.Rule.ID); err == nil {
		req.RuleID = parsedRuleID
	}

	if err := ar.dispatcher.Dispatch(&notify.DispatchJob{
		AlertID:      alertID,
		Alert:        m,
		Targets:      targets,
		Notification: req,
	}); err != nil {
		ar.log.Warn("dispatch failed", zap.Error(err), zap.String("alert_id", alertID.String()))
	}

	if ar.pool != nil {
		if err := ar.tryCorrelateIncident(ctx, alertID, appID, traceID, m.Rule.Severity); err != nil {
			ar.log.Warn("incident correlation failed", zap.Error(err), zap.String("alert_id", alertID.String()))
		}
	}

	return nil
}

func (ar *AlertRouter) upsertAlert(
	ctx context.Context,
	m engine.MatchResult,
	candidateID uuid.UUID,
	fingerprint string,
	signalID string,
	appID string,
	traceID string,
	layer int,
	isNew bool,
) (uuid.UUID, error) {
	if !isNew {
		var alertID uuid.UUID
		err := ar.pool.QueryRow(ctx, `
			UPDATE alerts
			   SET signal_count = signal_count + 1,
			       signal_ids = array_append(signal_ids, $1),
			       last_seen_at = now()
			 WHERE fingerprint = $2
			 RETURNING id
		`, signalID, fingerprint).Scan(&alertID)
		if err == nil {
			return alertID, nil
		}
		if err != pgx.ErrNoRows {
			return uuid.Nil, fmt.Errorf("update duplicate alert: %w", err)
		}
		// Fallback: treat as new if row disappeared between dedup and update.
	}

	var alertID uuid.UUID
	err := ar.pool.QueryRow(ctx, `
		INSERT INTO alerts (
			id, app_id, fingerprint, severity, layer, category, title, description, signal_ids, trace_id, status,
			signal_count, first_seen_at, last_seen_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'open',
			1, now(), now()
		)
		ON CONFLICT (fingerprint) DO UPDATE SET
			signal_count = alerts.signal_count + 1,
			signal_ids = array_append(alerts.signal_ids, EXCLUDED.signal_ids[1]),
			last_seen_at = now()
		RETURNING id
	`,
		candidateID,
		appID,
		fingerprint,
		m.Rule.Severity,
		layer,
		m.Signal.Category,
		m.Rule.Action.Title,
		m.Rule.Action.Description,
		[]string{signalID},
		traceID,
	).Scan(&alertID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert alert: %w", err)
	}
	return alertID, nil
}

func (ar *AlertRouter) tryCorrelateIncident(ctx context.Context, alertID uuid.UUID, appID, traceID string, severity int) error {
	if traceID == "" {
		return nil
	}

	var count int
	var alertIDs []uuid.UUID
	err := ar.pool.QueryRow(ctx, `
		SELECT count(*), COALESCE(array_agg(id), ARRAY[]::uuid[])
		FROM alerts
		WHERE app_id = $1
		  AND trace_id = $2
		  AND first_seen_at >= now() - interval '10 minute'
	`, appID, traceID).Scan(&count, &alertIDs)
	if err != nil {
		return err
	}
	if count < 3 {
		return nil
	}

	var incidentID uuid.UUID
	err = ar.pool.QueryRow(ctx, `
		SELECT id
		FROM incidents
		WHERE app_id = $1
		  AND status IN ('open', 'acknowledged')
		  AND trace_ids @> ARRAY[$2]::text[]
		ORDER BY created_at DESC
		LIMIT 1
	`, appID, traceID).Scan(&incidentID)
	if err == pgx.ErrNoRows {
		err = ar.pool.QueryRow(ctx, `
			INSERT INTO incidents (
				title, description, severity, app_id, status, alert_ids, trace_ids, created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, 'open', $5, $6, now(), now()
			)
			RETURNING id
		`,
			"Correlated alert cluster",
			"Auto-correlated incident from >=3 alerts in 10 minutes",
			severity,
			appID,
			alertIDs,
			[]string{traceID},
		).Scan(&incidentID)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	_, err = ar.pool.Exec(ctx, `
		UPDATE alerts
		SET incident_id = $1
		WHERE id = ANY($2::uuid[])
	`, incidentID, alertIDs)
	return err
}

func computeRouterFingerprint(ruleID, appID string) string {
	h := sha256.Sum256([]byte(ruleID + ":" + appID))
	return fmt.Sprintf("%x", h[:])
}
