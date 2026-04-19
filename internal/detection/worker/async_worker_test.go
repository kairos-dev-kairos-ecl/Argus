package worker

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/detection/breaker"
	"github.com/argusxdr/argus/internal/detection/engine"
	"github.com/argusxdr/argus/internal/kairos"
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

// stubAlertWriter records calls and the last kairos decision received.
type stubAlertWriter struct {
	calls          atomic.Int64
	lastDecision   *kairos.PolicyDecision
}

func (s *stubAlertWriter) WriteAlert(_ context.Context, _ engine.MatchResult, kd *kairos.PolicyDecision) error {
	s.calls.Add(1)
	s.lastDecision = kd
	return nil
}

// stubKairosEvaluator is a test double for KairosEvaluator.
type stubKairosEvaluator struct {
	calls     atomic.Int64
	returnErr error
	decision  *kairos.PolicyDecision
}

func (s *stubKairosEvaluator) Evaluate(_ context.Context, _ *kairos.EvaluationRequest) (*kairos.PolicyDecision, error) {
	s.calls.Add(1)
	if s.returnErr != nil {
		return nil, s.returnErr
	}
	return s.decision, nil
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

// gatherKairosTimeout returns the value of the kairos_timeout_total counter.
func gatherKairosTimeout(reg *prometheus.Registry) float64 {
	mfs, _ := reg.Gather()
	for _, mf := range mfs {
		if mf.GetName() == "kairos_timeout_total" {
			for _, m := range mf.GetMetric() {
				if m.Counter != nil && m.Counter.Value != nil {
					return *m.Counter.Value
				}
			}
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

// --- Kairos integration tests (D-12, D-13, D-14, D-15) ---

// TestAsyncWorker_KairosCalledWhenRequiresKairos verifies that when
// Rule.RequiresKairos=true the Kairos evaluator is invoked exactly once.
func TestAsyncWorker_KairosCalledWhenRequiresKairos(t *testing.T) {
	br := breaker.New(30 * time.Second)
	aw := &stubAlertWriter{}

	decision := &kairos.PolicyDecision{Decision: "allow", Confidence: 0.9}
	kStub := &stubKairosEvaluator{decision: decision}

	match := engine.MatchResult{
		Rule: engine.Rule{ID: "rule-kairos", Severity: 3, RequiresKairos: true},
		Tier: 2,
	}
	eng := &stubEngine{results: []engine.MatchResult{match}}

	_, m := newTestReg()
	// rand always 1.0 so sampling path is never triggered — only RequiresKairos matters
	w := newTestWorker(eng, aw, br, m, 100, func() float64 { return 1.0 })
	w.WithKairos(kStub)

	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)
	w.Enqueue(newSignal(v1.Severity_HIGH))

	require.Eventually(t, func() bool {
		return aw.calls.Load() == 1
	}, 2*time.Second, 10*time.Millisecond)

	cancel()
	w.Shutdown()

	assert.Equal(t, int64(1), kStub.calls.Load(), "Kairos must be called once")
	assert.Equal(t, decision, aw.lastDecision, "Kairos decision must reach alert writer")
}

// TestAsyncWorker_KairosSkippedByDefault verifies that when RequiresKairos=false,
// rand is always 1.0 (above sampling threshold), and no highRiskTagFn is set,
// the Kairos stub is NOT invoked. The alert is still written with nil decision.
func TestAsyncWorker_KairosSkippedByDefault(t *testing.T) {
	br := breaker.New(30 * time.Second)
	aw := &stubAlertWriter{}
	kStub := &stubKairosEvaluator{}

	match := engine.MatchResult{
		Rule: engine.Rule{ID: "rule-no-kairos", Severity: 3, RequiresKairos: false},
		Tier: 2,
	}
	eng := &stubEngine{results: []engine.MatchResult{match}}

	_, m := newTestReg()
	// rand=1.0 → sampling rate 0.02 < 1.0 so no sampling; RequiresKairos=false
	w := newTestWorker(eng, aw, br, m, 100, func() float64 { return 1.0 })
	w.WithKairos(kStub)

	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)
	w.Enqueue(newSignal(v1.Severity_HIGH))

	require.Eventually(t, func() bool {
		return aw.calls.Load() == 1
	}, 2*time.Second, 10*time.Millisecond)

	cancel()
	w.Shutdown()

	assert.Equal(t, int64(0), kStub.calls.Load(), "Kairos must NOT be called")
	assert.Nil(t, aw.lastDecision, "nil decision must reach alert writer when Kairos skipped")
}

// TestAsyncWorker_KairosTimeoutFailOpen verifies that a context.DeadlineExceeded from
// Kairos increments the KairosTimeout counter and the alert is still written (nil decision).
func TestAsyncWorker_KairosTimeoutFailOpen(t *testing.T) {
	br := breaker.New(30 * time.Second)
	aw := &stubAlertWriter{}
	kStub := &stubKairosEvaluator{returnErr: context.DeadlineExceeded}

	match := engine.MatchResult{
		Rule: engine.Rule{ID: "rule-timeout", Severity: 3, RequiresKairos: true},
		Tier: 2,
	}
	eng := &stubEngine{results: []engine.MatchResult{match}}

	reg, m := newTestReg()
	w := newTestWorker(eng, aw, br, m, 100, nil)
	w.WithKairos(kStub)

	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)
	w.Enqueue(newSignal(v1.Severity_HIGH))

	require.Eventually(t, func() bool {
		return aw.calls.Load() == 1
	}, 2*time.Second, 10*time.Millisecond)

	cancel()
	w.Shutdown()

	assert.Equal(t, float64(1), gatherKairosTimeout(reg), "KairosTimeout counter must be incremented")
	assert.Nil(t, aw.lastDecision, "nil decision must reach alert writer on timeout")
}

// TestAsyncWorker_KairosErrorFailOpen verifies that a generic Kairos error does not
// panic and results in an alert written with nil decision (fail-open, D-13).
func TestAsyncWorker_KairosErrorFailOpen(t *testing.T) {
	br := breaker.New(30 * time.Second)
	aw := &stubAlertWriter{}
	kStub := &stubKairosEvaluator{returnErr: errors.New("connection refused")}

	match := engine.MatchResult{
		Rule: engine.Rule{ID: "rule-error", Severity: 3, RequiresKairos: true},
		Tier: 2,
	}
	eng := &stubEngine{results: []engine.MatchResult{match}}

	_, m := newTestReg()
	w := newTestWorker(eng, aw, br, m, 100, nil)
	w.WithKairos(kStub)

	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)
	w.Enqueue(newSignal(v1.Severity_HIGH))

	require.Eventually(t, func() bool {
		return aw.calls.Load() == 1
	}, 2*time.Second, 10*time.Millisecond)

	cancel()
	w.Shutdown()

	assert.Nil(t, aw.lastDecision, "nil decision must reach alert writer on Kairos error")
}

// TestAsyncWorker_KairosSamplingWindow verifies the probabilistic sampling path:
// rand=0.01 < samplingRate=0.02 → Kairos invoked;
// rand=0.5  > samplingRate=0.02 + RequiresKairos=false → Kairos NOT invoked.
func TestAsyncWorker_KairosSamplingWindow(t *testing.T) {
	br := breaker.New(30 * time.Second)

	decision := &kairos.PolicyDecision{Decision: "allow", Confidence: 0.7}

	// Case 1: rand < samplingRate → Kairos invoked
	{
		aw := &stubAlertWriter{}
		kStub := &stubKairosEvaluator{decision: decision}
		match := engine.MatchResult{
			Rule: engine.Rule{ID: "rule-sample", Severity: 3, RequiresKairos: false},
			Tier: 2,
		}
		eng := &stubEngine{results: []engine.MatchResult{match}}
		_, m := newTestReg()
		w := newTestWorker(eng, aw, br, m, 100, func() float64 { return 0.01 })
		w.WithKairos(kStub).WithKairosSamplingRate(0.02)

		ctx, cancel := context.WithCancel(context.Background())
		w.Start(ctx)
		w.Enqueue(newSignal(v1.Severity_HIGH))

		require.Eventually(t, func() bool { return aw.calls.Load() == 1 }, 2*time.Second, 10*time.Millisecond)
		cancel()
		w.Shutdown()

		assert.Equal(t, int64(1), kStub.calls.Load(), "rand=0.01 < samplingRate=0.02: Kairos must be invoked")
	}

	// Case 2: rand > samplingRate, RequiresKairos=false → Kairos NOT invoked
	{
		aw := &stubAlertWriter{}
		kStub := &stubKairosEvaluator{decision: decision}
		match := engine.MatchResult{
			Rule: engine.Rule{ID: "rule-no-sample", Severity: 3, RequiresKairos: false},
			Tier: 2,
		}
		eng := &stubEngine{results: []engine.MatchResult{match}}
		_, m := newTestReg()
		w := newTestWorker(eng, aw, br, m, 100, func() float64 { return 0.5 })
		w.WithKairos(kStub).WithKairosSamplingRate(0.02)

		ctx, cancel := context.WithCancel(context.Background())
		w.Start(ctx)
		w.Enqueue(newSignal(v1.Severity_HIGH))

		require.Eventually(t, func() bool { return aw.calls.Load() == 1 }, 2*time.Second, 10*time.Millisecond)
		cancel()
		w.Shutdown()

		assert.Equal(t, int64(0), kStub.calls.Load(), "rand=0.5 > samplingRate=0.02: Kairos must NOT be invoked")
	}
}
