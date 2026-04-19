package worker

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/detection/breaker"
	"github.com/argusxdr/argus/internal/detection/engine"
	"github.com/argusxdr/argus/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// stubEngine returns a fixed set of MatchResults.
type stubEngine struct {
	results []engine.MatchResult
}

func (s *stubEngine) Evaluate(_ context.Context, sig *v1.ArgusSignal) ([]engine.MatchResult, error) {
	out := make([]engine.MatchResult, len(s.results))
	for i, r := range s.results {
		r.Signal = sig
		out[i] = r
	}
	return out, nil
}

// stubAlertWriter records calls.
type stubAlertWriter struct {
	calls atomic.Int64
}

func (s *stubAlertWriter) WriteAlert(_ context.Context, _ engine.MatchResult) error {
	s.calls.Add(1)
	return nil
}

func newTestReg() (*prometheus.Registry, *metrics.Detection) {
	reg := prometheus.NewRegistry()
	m := metrics.NewDetection(reg)
	return reg, m
}

func newTestWorker(eng Evaluator, aw AlertWriter, br *breaker.CircuitBreaker, m *metrics.Detection, queueSize int, randFn func() float64) *AsyncDetectionWorker {
	w := New(eng, aw, br, m, zap.NewNop(), queueSize, 1)
	if randFn != nil {
		w.rand = randFn
	}
	return w
}

func newSignal(sev v1.Severity) *v1.ArgusSignal {
	return &v1.ArgusSignal{
		SignalId: "test-signal",
		Severity: sev,
	}
}

// gatherDropped returns the total count across all "argus_detection_dropped_total" label combos.
func gatherDropped(reg *prometheus.Registry) float64 {
	mfs, _ := reg.Gather()
	for _, mf := range mfs {
		if mf.GetName() == "argus_detection_dropped_total" {
			var total float64
			for _, m := range mf.GetMetric() {
				if m.Counter != nil && m.Counter.Value != nil {
					total += *m.Counter.Value
				}
			}
			return total
		}
	}
	return 0
}

func TestAsyncWorker_DropLowWhenFull(t *testing.T) {
	br := breaker.New(30 * time.Second)
	aw := &stubAlertWriter{}
	reg, m := newTestReg()
	w := newTestWorker(&stubEngine{}, aw, br, m, 1, nil)

	// Fill the queue
	w.queue <- newSignal(v1.Severity_HIGH) // fill with non-low to avoid interference

	before := gatherDropped(reg)
	w.Enqueue(newSignal(v1.Severity_LOW))
	after := gatherDropped(reg)

	assert.Greater(t, after, before, "expected dropped counter to increment for LOW")
}

func TestAsyncWorker_NeverDropsCritical(t *testing.T) {
	br := breaker.New(30 * time.Second)
	aw := &stubAlertWriter{}
	_, m := newTestReg()
	w := newTestWorker(&stubEngine{}, aw, br, m, 1, nil)
	w.queue <- newSignal(v1.Severity_LOW) // fill queue

	done := make(chan struct{})
	go func() {
		w.Enqueue(newSignal(v1.Severity_CRITICAL))
		close(done)
	}()

	// Drain queue to unblock
	select {
	case <-w.queue:
	case <-time.After(100 * time.Millisecond):
	}

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("CRITICAL signal was not enqueued within timeout")
	}
}

func TestAsyncWorker_SamplingWhenBreakerOpen(t *testing.T) {
	br := breaker.New(30 * time.Second)
	br.Evaluate(900, 1000, 0, 500) // trip breaker

	aw := &stubAlertWriter{}

	// rand=1.0 → above 0.10 threshold → skipped
	_, m1 := newTestReg()
	w := newTestWorker(&stubEngine{}, aw, br, m1, 100, func() float64 { return 1.0 })
	initialLen := len(w.queue)
	w.Enqueue(newSignal(v1.Severity_HIGH))
	assert.Equal(t, initialLen, len(w.queue), "signal should be sampled out (rand=1.0)")

	// rand=0.05 → below 0.10 threshold → passes through
	_, m2 := newTestReg()
	w2 := newTestWorker(&stubEngine{}, aw, br, m2, 100, func() float64 { return 0.05 })
	w2.Enqueue(newSignal(v1.Severity_HIGH))
	assert.Equal(t, 1, len(w2.queue), "signal should be enqueued (rand=0.05)")
}

func TestAsyncWorker_EvaluatesAndWritesAlert(t *testing.T) {
	br := breaker.New(30 * time.Second)
	aw := &stubAlertWriter{}

	match := engine.MatchResult{
		Rule:   engine.Rule{ID: "rule-1", Severity: 3},
		Tier:   2,
		Reason: "tier2",
	}
	eng := &stubEngine{results: []engine.MatchResult{match}}

	_, m := newTestReg()
	w := newTestWorker(eng, aw, br, m, 100, nil)

	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)

	w.Enqueue(newSignal(v1.Severity_HIGH))

	require.Eventually(t, func() bool {
		return aw.calls.Load() == 1
	}, 2*time.Second, 10*time.Millisecond, "expected exactly 1 alert written")

	cancel()
	w.Shutdown()
}
