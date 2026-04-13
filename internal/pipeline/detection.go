package pipeline

import (
	"context"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/detection/engine"
	"go.uber.org/zap"
)

// AlertWriter persists a matched detection result as an alert.
type AlertWriter interface {
	WriteAlert(ctx context.Context, match engine.MatchResult) error
}

// DetectionProcessor evaluates signals against detection rules and writes alerts.
type DetectionProcessor struct {
	engine *engine.DetectionEngine
	writer AlertWriter
	log    *zap.Logger
}

// NewDetectionProcessor creates a DetectionProcessor.
func NewDetectionProcessor(e *engine.DetectionEngine, writer AlertWriter) *DetectionProcessor {
	if writer == nil {
		writer = noopAlertWriter{}
	}
	return &DetectionProcessor{
		engine: e,
		writer: writer,
		log:    zap.L(),
	}
}

// SetLogger overrides the default logger.
func (d *DetectionProcessor) SetLogger(log *zap.Logger) {
	if log != nil {
		d.log = log
	}
}

// Process evaluates the signal and writes alerts for any matches.
func (d *DetectionProcessor) Process(ctx context.Context, sig *v1.ArgusSignal) (*v1.ArgusSignal, error) {
	if sig == nil {
		return nil, nil
	}
	if d.engine == nil {
		return sig, nil
	}

	matches, err := d.engine.Evaluate(ctx, sig)
	if err != nil {
		d.log.Warn("detection engine error (non-fatal)",
			zap.String("signal_id", sig.SignalId),
			zap.Error(err),
		)
		return sig, nil
	}

	for _, m := range matches {
		if err := d.writer.WriteAlert(ctx, m); err != nil {
			d.log.Warn("alert write failed (non-fatal)",
				zap.String("signal_id", sig.SignalId),
				zap.String("rule_id", m.Rule.ID),
				zap.Error(err),
			)
		}
	}
	return sig, nil
}

// Name implements Processor.
func (d *DetectionProcessor) Name() string {
	return "Detection"
}

type noopAlertWriter struct{}

func (noopAlertWriter) WriteAlert(_ context.Context, _ engine.MatchResult) error {
	return nil
}
