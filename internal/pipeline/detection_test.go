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
	assert.Equal(t, 1, w.written)
}

func TestDetectionProcessorNilSignalReturnsNil(t *testing.T) {
	store := engine.NewRuleStore()
	e := engine.New(store, nil)
	proc := pipeline.NewDetectionProcessor(e, &testAlertWriter{})
	out, err := proc.Process(context.Background(), nil)
	require.NoError(t, err)
	assert.Nil(t, out)
}
