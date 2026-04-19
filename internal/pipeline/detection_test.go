package pipeline_test

import (
	"context"
	"testing"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/detection/engine"
	"github.com/argusxdr/argus/internal/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type testAlertWriter struct {
	written int
}

func (w *testAlertWriter) WriteAlert(_ context.Context, _ engine.MatchResult) error {
	w.written++
	return nil
}

func TestDetectionProcessorNoRules(t *testing.T) {
	store := engine.NewRuleStore()
	e := engine.New(store, nil)
	w := &testAlertWriter{}

	proc := pipeline.NewDetectionProcessor(e, w)
	sig := &v1.ArgusSignal{
		SignalId:   "s1",
		Layer:      v1.Layer_L6_SAFETY,
		Category:   "safety.test",
		Timestamp:  timestamppb.Now(),
		IngestedAt: timestamppb.Now(),
		Source:     &v1.Source{AppId: "app-001"},
	}

	out, err := proc.Process(context.Background(), sig)
	require.NoError(t, err)
	assert.Equal(t, sig, out)
	assert.Equal(t, 0, w.written)
}

func TestDetectionProcessorWritesAlertOnMatch(t *testing.T) {
	// DetectionProcessor no longer writes alerts inline (per Task 2).
	// It only annotates the signal with matched Tier 1 rule IDs.
	store := engine.NewRuleStore()
	store.Add(engine.Rule{
		ID: "r1", Name: "R", Tier: 1, Enabled: true, Severity: 4,
		Conditions: engine.Conditions{Layer: "L6_SAFETY"},
		Action:     engine.Action{Title: "Test Alert"},
	})
	e := engine.New(store, nil)
	w := &testAlertWriter{}

	proc := pipeline.NewDetectionProcessor(e, w)
	sig := &v1.ArgusSignal{
		SignalId:   "s1",
		Layer:      v1.Layer_L6_SAFETY,
		Category:   "safety.test",
		Timestamp:  timestamppb.Now(),
		IngestedAt: timestamppb.Now(),
		Source:     &v1.Source{AppId: "app-001"},
	}

	out, err := proc.Process(context.Background(), sig)
	require.NoError(t, err)
	assert.Equal(t, sig, out)
	// Alert is NOT written inline; the async worker handles this
	assert.Equal(t, 0, w.written)
	assert.Contains(t, out.RelatedSignals, "rule:r1")
}

func TestDetectionProcessor_Tier1Only(t *testing.T) {
	// A rule store with only a Tier 3 rule should yield no tags on the signal
	store := engine.NewRuleStore()
	store.Add(engine.Rule{
		ID: "t3-rule", Name: "Tier3", Tier: 3, Enabled: true, Severity: 4,
		Conditions: engine.Conditions{
			Temporal: &engine.TemporalCond{CountGte: 1, WindowSeconds: 60},
		},
		Action: engine.Action{Title: "Tier3 Alert"},
	})
	e := engine.New(store, nil)
	w := &testAlertWriter{}

	proc := pipeline.NewDetectionProcessor(e, w)
	sig := &v1.ArgusSignal{
		SignalId:   "s-tier3",
		Layer:      v1.Layer_L6_SAFETY,
		Category:   "safety.test",
		Timestamp:  timestamppb.Now(),
		IngestedAt: timestamppb.Now(),
		Source:     &v1.Source{AppId: "app-001"},
	}

	out, err := proc.Process(context.Background(), sig)
	require.NoError(t, err)
	assert.Equal(t, sig, out)
	// Tier 3 rule is NOT evaluated inline — no alert writes
	assert.Equal(t, 0, w.written, "Tier3 rule must not trigger alert write in pipeline")
}

func TestDetectionProcessor_TagsTier1Matches(t *testing.T) {
	store := engine.NewRuleStore()
	store.Add(engine.Rule{
		ID: "t1-rule", Name: "Tier1", Tier: 1, Enabled: true, Severity: 3,
		Conditions: engine.Conditions{Layer: "L6_SAFETY"},
		Action:     engine.Action{Title: "Tier1 Alert"},
	})
	e := engine.New(store, nil)
	w := &testAlertWriter{}

	proc := pipeline.NewDetectionProcessor(e, w)
	sig := &v1.ArgusSignal{
		SignalId:   "s-tier1",
		Layer:      v1.Layer_L6_SAFETY,
		Category:   "safety.test",
		Timestamp:  timestamppb.Now(),
		IngestedAt: timestamppb.Now(),
		Source:     &v1.Source{AppId: "app-001"},
	}

	out, err := proc.Process(context.Background(), sig)
	require.NoError(t, err)
	require.NotNil(t, out)
	// Tier 1 match annotates signal; no alert write from processor
	assert.Equal(t, 0, w.written, "DetectionProcessor must not write alerts inline")
	// Rule IDs are recorded in RelatedSignals (proto proxy until MatchedRuleIds is added to schema)
	assert.Contains(t, out.RelatedSignals, "rule:t1-rule")
}

func TestDetectionProcessorNilSignalReturnsNil(t *testing.T) {
	store := engine.NewRuleStore()
	e := engine.New(store, nil)
	proc := pipeline.NewDetectionProcessor(e, &testAlertWriter{})
	out, err := proc.Process(context.Background(), nil)
	require.NoError(t, err)
	assert.Nil(t, out)
}
