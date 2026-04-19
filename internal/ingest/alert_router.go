package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/argusxdr/argus/internal/detection/engine"
	"github.com/argusxdr/argus/internal/kairos"
	"github.com/argusxdr/argus/internal/notify"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
)

const dedupWindow = 15 * time.Minute

// defaultSuppressThreshold is the signal_count at which an alert is auto-suppressed.
const defaultSuppressThreshold = 100

// ErrMissingTraceID is returned when WriteAlert is called without a trace_id (D-06).
var ErrMissingTraceID = errors.New("alert rejected: trace_id is required (D-06)")

type routingEvaluator interface {
	Evaluate(ctx *notify.EvaluationContext) []string
}

type dispatchSink interface {
	Dispatch(job *notify.DispatchJob) error
}

// AlertRouter is the post-detection pipeline owner: dedup -> persist -> route -> dispatch.
type AlertRouter struct {
	pool               *pgxpool.Pool
	redis              *redis.Client
	routing            routingEvaluator
	dispatcher         dispatchSink
	log                *zap.Logger
	suppressThreshold  int
}

// NewAlertRouter creates a new alert router.
func NewAlertRouter(pool *pgxpool.Pool, redisClient *redis.Client, routing routingEvaluator, dispatcher dispatchSink, log *zap.Logger) *AlertRouter {
	if log == nil {
		log = zap.NewNop()
	}
	return &AlertRouter{
		pool:              pool,
		redis:             redisClient,
		routing:           routing,
		dispatcher:        dispatcher,
		log:               log,
		suppressThreshold: defaultSuppressThreshold,
	}
}

// WriteAlert implements worker.AlertWriter.
// kairosDecision may be nil (Kairos disabled, timed out, or failed — fail-open).
// Returns ErrMissingTraceID if sig.TraceId is empty (D-06); no DB write is attempted.
func (ar *AlertRouter) WriteAlert(ctx context.Context, m engine.MatchResult, kairosDecision *kairos.PolicyDecision) error {
	if m.Signal == nil {
		return nil
	}

	// D-06: trace linkage is mandatory.
	if m.Signal.TraceId == "" {
		ar.log.Warn("alert rejected, missing trace_id", zap.String("rule_id", m.Rule.ID))
		return ErrMissingTraceID
	}

	appID := ""
	if m.Signal.Source != nil {
		appID = m.Signal.Source.AppId
	}
	traceID := m.Signal.TraceId
	signalID := m.Signal.SignalId
	layer := int(m.Signal.Layer)
	fingerprint := computeRouterFingerprint(m.Rule.ID, m.Signal)

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
		alertID, err = ar.upsertAlert(ctx, m, kairosDecision, alertID, fingerprint, signalID, appID, traceID, layer)
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
	kairosDecision *kairos.PolicyDecision,
	candidateID uuid.UUID,
	fingerprint string,
	signalID string,
	appID string,
	traceID string,
	layer int,
) (uuid.UUID, error) {
	// Build JSONB context from rule action (D-07).
	ctxJSON, _ := json.Marshal(map[string]any{
		"rule_action": m.Rule.Action,
		"tier":        m.Tier,
		"reason":      m.Reason,
	})
	// kairos_decision: persist Kairos policy decision if provided, else null.
	var kairosJSON []byte
	if kairosDecision != nil {
		kairosJSON, _ = json.Marshal(kairosDecision)
	} else {
		kairosJSON = []byte("null")
	}

	var alertID uuid.UUID
	var signalCount int
	err := ar.pool.QueryRow(ctx, `
		INSERT INTO alerts (
			id, rule_id, app_id, trace_id, signal_ids, signal_count,
			fingerprint, severity, layer, category, title, description,
			status, context, kairos_decision, first_seen_at, last_seen_at
		) VALUES (
			$1, $2, $3, $4, $5, 1,
			$6, $7, $8, $9, $10, $11,
			'open', $12, $13, NOW(), NOW()
		)
		ON CONFLICT (fingerprint) DO UPDATE SET
			signal_count    = alerts.signal_count + 1,
			last_seen_at    = NOW(),
			signal_ids      = array_append(alerts.signal_ids, $14),
			kairos_decision = COALESCE(EXCLUDED.kairos_decision, alerts.kairos_decision)
		RETURNING id, signal_count
	`,
		candidateID,
		m.Rule.ID,
		appID,
		traceID,
		[]string{signalID},
		fingerprint,
		m.Rule.Severity,
		layer,
		m.Signal.Category,
		m.Rule.Action.Title,
		m.Rule.Action.Description,
		ctxJSON,
		kairosJSON,
		signalID, // $14 for array_append on conflict
	).Scan(&alertID, &signalCount)
	if err != nil {
		return uuid.Nil, fmt.Errorf("upsert alert: %w", err)
	}

	// D-04: Auto-suppress when dedup count exceeds threshold.
	threshold := ar.suppressThreshold
	if threshold <= 0 {
		threshold = defaultSuppressThreshold
	}
	if signalCount > threshold {
		_, suppErr := ar.pool.Exec(ctx,
			`UPDATE alerts SET status='suppressed' WHERE id=$1 AND status='open'`, alertID)
		if suppErr == nil {
			ar.log.Info("alert auto-suppressed",
				zap.String("alert_id", alertID.String()),
				zap.Int("signal_count", signalCount))
		}
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

// computeRouterFingerprint returns sha256 hex over: rule_id + "|" + entity + "|" + normalized_payload.
// entity = Source.AppId + ":" + SessionId (most specific per-caller identifier available).
// normalized_payload = Layer + "|" + Category + "|" + Severity (stable, deterministic).
func computeRouterFingerprint(ruleID string, sig *v1.ArgusSignal) string {
	appID := ""
	if sig.Source != nil {
		appID = sig.Source.AppId
	}
	sessionID := sig.GetSessionId()
	entity := fmt.Sprintf("%s:%s", appID, sessionID)
	payload := fmt.Sprintf("%d|%s|%d", sig.Layer, sig.Category, sig.Severity)
	h := sha256.Sum256([]byte(ruleID + "|" + entity + "|" + payload))
	return hex.EncodeToString(h[:])
}
