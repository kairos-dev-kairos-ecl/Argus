package pipeline

import (
	"context"
	"fmt"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/detection/engine"
	"go.uber.org/zap"
)

// DetectionProcessor evaluates signals against Tier 1 detection rules inline.
// Tier 2/3 evaluation is handled asynchronously by the detection worker.
// This processor does NOT write alerts; it only annotates signals with matched rule IDs.
type DetectionProcessor struct {
	engine *engine.DetectionEngine
	log    *zap.Logger
}

// NewDetectionProcessor creates a DetectionProcessor.
// The writer parameter is accepted for backward compatibility but is no longer used.
// Alert writing is handled by the async detection worker.
func NewDetectionProcessor(e *engine.DetectionEngine, _ AlertWriter) *DetectionProcessor {
	return &DetectionProcessor{
		engine: e,
		log:    zap.L(),
	}
}

// SetLogger overrides the default logger.
func (d *DetectionProcessor) SetLogger(log *zap.Logger) {
	if log != nil {
		d.log = log
	}
}

// Process evaluates the signal against Tier 1 rules only.
// On Tier 1 matches, matched rule IDs are recorded in sig.RelatedSignals with a "rule:" prefix.
// No alert writes occur in this processor.
func (d *DetectionProcessor) Process(ctx context.Context, sig *v1.ArgusSignal) (*v1.ArgusSignal, error) {
	if sig == nil {
		return nil, nil
	}
	if d.engine == nil {
		return sig, nil
	}

	matches, err := d.engine.EvaluateTier1(ctx, sig)
	if err != nil {
		d.log.Warn("detection engine error (non-fatal)",
			zap.String("signal_id", sig.SignalId),
			zap.Error(err),
		)
		return sig, nil
	}

	if len(matches) == 0 {
		return sig, nil
	}

	// Annotate signal with matched rule IDs (Tier 1 only).
	// Rule IDs are stored in RelatedSignals with a "rule:" prefix until the proto
	// gains a dedicated matched_rule_ids field.
	for _, m := range matches {
		tag := fmt.Sprintf("rule:%s", m.Rule.ID)
		sig.RelatedSignals = append(sig.RelatedSignals, tag)
	}

	ruleIDs := make([]string, len(matches))
	for i, m := range matches {
		ruleIDs[i] = m.Rule.ID
	}
	d.log.Debug("tier1 rules matched",
		zap.String("signal_id", sig.SignalId),
		zap.Strings("rule_ids", ruleIDs),
	)

	return sig, nil
}

// Name implements Processor.
func (d *DetectionProcessor) Name() string {
	return "Detection"
}

// AlertWriter is kept for interface compatibility but is no longer used by DetectionProcessor.
// Alert writing is handled by the async detection worker.
type AlertWriter interface {
	WriteAlert(ctx context.Context, match engine.MatchResult) error
}

type noopAlertWriter struct{}

func (noopAlertWriter) WriteAlert(_ context.Context, _ engine.MatchResult) error {
	return nil
}
