// Package worker provides the async detection worker with severity-based drop
// policy, circuit-breaker integration, and Prometheus metrics.
package worker

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"time"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/detection/breaker"
	"github.com/argusxdr/argus/internal/detection/engine"
	"github.com/argusxdr/argus/internal/metrics"
	"go.uber.org/zap"
)

// Evaluator is satisfied by *engine.DetectionEngine.
// Kept as an interface so tests can inject stubs.
type Evaluator interface {
	Evaluate(ctx context.Context, sig *v1.ArgusSignal) ([]engine.MatchResult, error)
}

// AlertWriter persists a matched detection result as an alert.
// Matches pipeline.AlertWriter and ingest.AlertRouter signatures.
type AlertWriter interface {
	WriteAlert(ctx context.Context, m engine.MatchResult) error
}

// AsyncDetectionWorker is the async detection queue with severity-based drop
// policy and circuit-breaker integration. Implements D-16, D-17, D-19.
type AsyncDetectionWorker struct {
	queue       chan *v1.ArgusSignal
	engine      Evaluator
	alertWriter AlertWriter
	breaker     *breaker.CircuitBreaker
	metrics     *metrics.Detection
	log         *zap.Logger
	workerCount int

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
		queue:       make(chan *v1.ArgusSignal, queueSize),
		engine:      eng,
		alertWriter: aw,
		breaker:     br,
		metrics:     m,
		log:         log,
		workerCount: workerCount,
		rand:        rand.Float64,
	}
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
		if err := w.alertWriter.WriteAlert(ctx, m); err != nil {
			w.log.Warn("alert write error", zap.Error(err), zap.String("signal_id", sig.SignalId))
			continue
		}
		severityLabel := SeverityLabel(int(sig.Severity))
		w.metrics.AlertsCreated.WithLabelValues(severityLabel, m.Rule.ID).Inc()
	}
}
