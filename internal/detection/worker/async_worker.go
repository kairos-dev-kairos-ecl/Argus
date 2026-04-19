// Package worker provides the async detection worker with severity-based drop
// policy, circuit-breaker integration, and Prometheus metrics.
package worker

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"time"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/detection/breaker"
	"github.com/argusxdr/argus/internal/detection/engine"
	"github.com/argusxdr/argus/internal/kairos"
	"github.com/argusxdr/argus/internal/metrics"
	"go.uber.org/zap"
)

// Evaluator is satisfied by *engine.DetectionEngine.
// Kept as an interface so tests can inject stubs.
type Evaluator interface {
	Evaluate(ctx context.Context, sig *v1.ArgusSignal) ([]engine.MatchResult, error)
}

// KairosEvaluator is the interface for Kairos policy evaluation.
// Satisfied by *kairos.Evaluator; kept as interface for testability.
// A nil KairosEvaluator means Kairos is disabled for this worker.
type KairosEvaluator interface {
	Evaluate(ctx context.Context, req *kairos.EvaluationRequest) (*kairos.PolicyDecision, error)
}

// AlertWriter persists a matched detection result as an alert, optionally
// including a Kairos policy decision (nil when Kairos was not called or failed).
type AlertWriter interface {
	WriteAlert(ctx context.Context, m engine.MatchResult, kairosDecision *kairos.PolicyDecision) error
}

// AsyncDetectionWorker is the async detection queue with severity-based drop
// policy and circuit-breaker integration. Implements D-12, D-13, D-14, D-15, D-16, D-17, D-19.
type AsyncDetectionWorker struct {
	queue       chan *v1.ArgusSignal
	engine      Evaluator
	alertWriter AlertWriter
	breaker     *breaker.CircuitBreaker
	metrics     *metrics.Detection
	log         *zap.Logger
	workerCount int

	// Kairos integration (D-12/D-13/D-14/D-15)
	kairosEval    KairosEvaluator // nil means Kairos disabled
	samplingRate  float64         // fraction 0.0-1.0; default 0.02 (2%)
	highRiskTagFn func(*v1.ArgusSignal) bool // optional; nil = not used
	kairosTimeout time.Duration   // default 500ms

	// rand is injectable for deterministic tests (default: math/rand.Float64).
	rand func() float64

	wg     sync.WaitGroup
	cancel context.CancelFunc
}

// New creates a new AsyncDetectionWorker.
// queueSize defaults to 10000 if <= 0; workerCount defaults to GOMAXPROCS*2 if <= 0.
func New(
	eng Evaluator,
	aw AlertWriter,
	br *breaker.CircuitBreaker,
	m *metrics.Detection,
	log *zap.Logger,
	queueSize, workerCount int,
) *AsyncDetectionWorker {
	if queueSize <= 0 {
		queueSize = 10_000
	}
	if workerCount <= 0 {
		workerCount = runtime.GOMAXPROCS(0) * 2
	}
	if log == nil {
		log = zap.NewNop()
	}
	return &AsyncDetectionWorker{
		queue:         make(chan *v1.ArgusSignal, queueSize),
		engine:        eng,
		alertWriter:   aw,
		breaker:       br,
		metrics:       m,
		log:           log,
		workerCount:   workerCount,
		samplingRate:  0.02,
		kairosTimeout: 500 * time.Millisecond,
		rand:          rand.Float64,
	}
}

// WithKairos attaches a KairosEvaluator to the worker (optional).
func (w *AsyncDetectionWorker) WithKairos(eval KairosEvaluator) *AsyncDetectionWorker {
	w.kairosEval = eval
	return w
}

// WithKairosSamplingRate sets the sampling rate for Kairos calls (default 0.02).
func (w *AsyncDetectionWorker) WithKairosSamplingRate(rate float64) *AsyncDetectionWorker {
	w.samplingRate = rate
	return w
}

// WithHighRiskTagFn sets a function to detect high-risk signals for Kairos escalation.
func (w *AsyncDetectionWorker) WithHighRiskTagFn(fn func(*v1.ArgusSignal) bool) *AsyncDetectionWorker {
	w.highRiskTagFn = fn
	return w
}

// Start spawns the worker goroutines. ctx cancellation begins graceful shutdown.
func (w *AsyncDetectionWorker) Start(ctx context.Context) {
	innerCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel

	for i := 0; i < w.workerCount; i++ {
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			w.runOne(innerCtx)
		}()
	}
}

// Shutdown cancels the worker context and waits for all goroutines to exit.
func (w *AsyncDetectionWorker) Shutdown() {
	if w.cancel != nil {
		w.cancel()
	}
	w.wg.Wait()
}

// Enqueue applies D-16/D-17/D-19 drop and sampling logic before writing to the
// internal queue.
//
//   - If breaker.IsOpen(), only 10% of traffic passes (sampling mode).
//   - Queue full + LOW severity  → drop, increment metric.
//   - Queue full + MEDIUM severity → 50% probability drop.
//   - Queue full + HIGH/CRITICAL  → block until queue has space (never dropped).
func (w *AsyncDetectionWorker) Enqueue(sig *v1.ArgusSignal) {
	// D-19: breaker sampling — when OPEN, only 10% passes through
	if w.breaker.IsOpen() && w.rand() > 0.10 {
		return
	}

	sevInt := SeverityInt(sig.Severity)

	select {
	case w.queue <- sig:
		w.metrics.QueueDepth.Set(float64(len(w.queue)))

	default:
		// Queue is full — apply severity-based drop policy (D-16, D-17)
		switch {
		case sevInt <= 2: // LOW (2) and INFO (1) — always drop
			label := SeverityLabel(sevInt)
			w.metrics.Dropped.WithLabelValues(label).Inc()
			w.log.Warn("dropped signal: queue full",
				zap.String("severity", label),
				zap.String("signal_id", sig.SignalId),
			)

		case sevInt == 3: // MEDIUM — 50% drop
			if w.rand() < 0.50 {
				w.metrics.Dropped.WithLabelValues("medium").Inc()
				return
			}
			// Survivor: blocking send
			w.queue <- sig
			w.metrics.QueueDepth.Set(float64(len(w.queue)))

		default: // HIGH (4) / CRITICAL (5) — never drop, block until enqueued
			w.queue <- sig
			w.metrics.QueueDepth.Set(float64(len(w.queue)))
		}
	}
}

// runOne is the inner worker loop. Each goroutine runs this.
func (w *AsyncDetectionWorker) runOne(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			// Drain remaining signals
			for {
				select {
				case sig := <-w.queue:
					w.process(ctx, sig)
				default:
					return
				}
			}
		case sig := <-w.queue:
			w.process(ctx, sig)
		}
	}
}

func (w *AsyncDetectionWorker) process(ctx context.Context, sig *v1.ArgusSignal) {
	start := time.Now()
	tierLabel := fmt.Sprintf("%d", sig.Layer)

	matches, err := w.engine.Evaluate(ctx, sig)
	if err != nil {
		w.log.Warn("detection evaluate error", zap.Error(err), zap.String("signal_id", sig.SignalId))
		return
	}

	latency := time.Since(start).Seconds()
	w.metrics.Latency.WithLabelValues(tierLabel).Observe(latency)
	w.metrics.Evaluations.WithLabelValues(tierLabel).Inc()

	for _, m := range matches {
		decision := w.callKairos(ctx, m, sig)

		if err := w.alertWriter.WriteAlert(ctx, m, decision); err != nil {
			w.log.Warn("alert write error", zap.Error(err), zap.String("signal_id", sig.SignalId))
			continue
		}
		severityLabel := SeverityLabel(int(sig.Severity))
		w.metrics.AlertsCreated.WithLabelValues(severityLabel, m.Rule.ID).Inc()
	}
}

// callKairos conditionally evaluates the Kairos policy for a match.
// Returns nil if Kairos is not configured, not selected, timed out, or errors (fail-open, D-13/D-14).
// Kairos is only called asynchronously within the worker — never on the ingest hot path (D-12).
func (w *AsyncDetectionWorker) callKairos(ctx context.Context, m engine.MatchResult, sig *v1.ArgusSignal) *kairos.PolicyDecision {
	if w.kairosEval == nil {
		return nil
	}
	if !w.shouldCallKairos(m, sig) {
		return nil
	}

	kctx, cancel := context.WithTimeout(ctx, w.kairosTimeout)
	defer cancel()

	req := kairos.BuildRequest(m.Rule.ID, sig)
	d, err := w.kairosEval.Evaluate(kctx, req)
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		w.metrics.KairosTimeout.Inc()
		w.log.Warn("kairos timeout — fail-open", zap.String("rule_id", m.Rule.ID))
		return nil
	case err != nil:
		w.log.Warn("kairos unreachable — fail-open", zap.Error(err), zap.String("rule_id", m.Rule.ID))
		return nil
	}
	return d
}

// shouldCallKairos returns true when the match or signal qualifies for Kairos evaluation.
// Conditions (any one sufficient):
//   - Rule has RequiresKairos=true (D-15)
//   - Random sampling within samplingRate (1-5% window)
//   - highRiskTagFn is set and returns true for this signal
func (w *AsyncDetectionWorker) shouldCallKairos(m engine.MatchResult, sig *v1.ArgusSignal) bool {
	if m.Rule.RequiresKairos {
		return true
	}
	if w.rand() < w.samplingRate {
		return true
	}
	if w.highRiskTagFn != nil && w.highRiskTagFn(sig) {
		return true
	}
	return false
}
