package adapters

import (
	"context"
	"fmt"
	"time"

	"github.com/argusxdr/argus/internal/notify"
	"github.com/PagerDuty/go-pagerduty"
	"go.uber.org/zap"
)

// PagerDutyConfig contains the configuration for the PagerDuty notifier.
type PagerDutyConfig struct {
	IntegrationKey string // Events API v2 routing key
	ServiceID      string // Optional: PagerDuty service ID for escalation
}

// PagerDutyNotifier sends notifications to PagerDuty using the go-pagerduty SDK.
type PagerDutyNotifier struct {
	config PagerDutyConfig
	client *pagerduty.Client
	logger *zap.Logger
}

// NewPagerDutyNotifier creates a new PagerDuty notifier with the given configuration.
func NewPagerDutyNotifier(config PagerDutyConfig, logger *zap.Logger) (*PagerDutyNotifier, error) {
	if config.IntegrationKey == "" {
		return nil, fmt.Errorf("PagerDuty integration key is required")
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &PagerDutyNotifier{
		config: config,
		client: pagerduty.NewClient(""), // Events API doesn't use token auth, uses routing key
		logger: logger,
	}, nil
}

// Name returns the name of this notifier.
func (p *PagerDutyNotifier) Name() string {
	return "pagerduty"
}

// pdSeverityMapping maps alert severity to PagerDuty severity levels.
func pdSeverityMapping(severity int) string {
	switch severity {
	case 1:
		return "info"
	case 2:
		return "warning"
	case 3:
		return "error"
	case 4, 5:
		return "critical"
	default:
		return "error"
	}
}

// Send sends an incident to PagerDuty with dedup_key for idempotence.
func (p *PagerDutyNotifier) Send(ctx context.Context, req *notify.NotificationRequest) (*notify.NotificationResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("notification request cannot be nil")
	}

	// Use fingerprint as dedup_key to ensure same alert doesn't create duplicate incidents
	dedupKey := req.Metadata["fingerprint"]
	if dedupKey == "" {
		dedupKey = fmt.Sprintf("%s-%s", req.RuleID.String(), req.AlertID.String())
	}

	// Construct event details
	eventDetails := map[string]interface{}{
		"alert_id":   req.AlertID.String(),
		"rule_id":    req.RuleID.String(),
		"signal_ids": req.Metadata["signal_ids"],
		"confidence": req.Metadata["confidence"],
	}

	// Create PagerDuty V2 event
	event := pagerduty.V2Event{
		RoutingKey: p.config.IntegrationKey,
		Action:     "trigger",
		DedupKey:   dedupKey,
		Payload: &pagerduty.V2Payload{
			Summary:   fmt.Sprintf("%s: %d signals detected", req.Title, countSignals(req.Metadata["signal_ids"])),
			Timestamp: time.Now().Format(time.RFC3339),
			Severity:  pdSeverityMapping(req.Severity),
			Source:    "Argus XDR",
			Details:   eventDetails,
		},
	}

	// Create context with timeout
	pdCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Send event
	resp, err := pagerduty.ManageEventWithContext(pdCtx, event)
	if err != nil {
		p.logger.Error("failed to send PagerDuty event", zap.Error(err), zap.String("alert_id", req.AlertID.String()))
		return &notify.NotificationResponse{
			Status:    "failed",
			Error:     err.Error(),
			Timestamp: time.Now().Unix(),
		}, err
	}

	p.logger.Info("PagerDuty event sent", zap.String("alert_id", req.AlertID.String()), zap.String("dedup_key", dedupKey), zap.String("status", resp.Status))

	return &notify.NotificationResponse{
		Status:    "sent",
		MessageID: resp.DedupKey,
		Timestamp: time.Now().Unix(),
	}, nil
}

// countSignals counts the number of signals in the comma-separated signal_ids string.
func countSignals(signalIDs string) int {
	if signalIDs == "" {
		return 0
	}
	count := 0
	for _, c := range signalIDs {
		if c == ',' {
			count++
		}
	}
	return count + 1
}

// Health checks if PagerDuty API is accessible.
// For Events API v2, we just verify basic connectivity.
func (p *PagerDutyNotifier) Health(ctx context.Context) error {
	// Events API v2 doesn't require authentication for health checks in the traditional sense.
	// We'll do a minimal validation by checking if the routing key is configured.
	if p.config.IntegrationKey == "" {
		return fmt.Errorf("pagerduty health check failed: integration key not configured")
	}
	return nil
}
