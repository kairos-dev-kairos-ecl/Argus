package engine_test

import (
	"context"
	"testing"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/detection/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestEngineEvaluate(t *testing.T) {
	store := engine.NewRuleStore()
	store.Add(engine.Rule{
		ID: "r1", Name: "L6 Match", Tier: 1, Enabled: true, Severity: 3,
		Conditions: engine.Conditions{Layer: "L6_SAFETY"},
		Action:     engine.Action{Title: "Tier1"},
	})

	eng := engine.New(store, nil)
	sig := &v1.ArgusSignal{
		SignalId:   "s1",
		Layer:      v1.Layer_L6_SAFETY,
		Category:   "safety.test",
		Severity:   v1.Severity_HIGH,
		Timestamp:  timestamppb.Now(),
		IngestedAt: timestamppb.Now(),
		Source:     &v1.Source{AppId: "app-001"},
	}

	matches, err := eng.Evaluate(context.Background(), sig)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, "r1", matches[0].Rule.ID)
	assert.Equal(t, 1, matches[0].Tier)
}
