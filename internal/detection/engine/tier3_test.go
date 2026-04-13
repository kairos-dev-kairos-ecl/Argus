package engine_test

import (
	"context"
	"testing"
	"time"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/detection/engine"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakeTemporalStore struct {
	count int
	err   error
}

func (f *fakeTemporalStore) AddAndCount(_ context.Context, _ string, _ time.Time, _ int) (int, error) {
	return f.count, f.err
}

func TestTier3Matches(t *testing.T) {
	rule := engine.Rule{
		ID: "t3-1", Name: "Burst", Tier: 3, Severity: 4,
		Conditions: engine.Conditions{
			Temporal: &engine.TemporalCond{CountGte: 5, WindowSeconds: 300},
		},
		Action: engine.Action{Title: "Burst Alert"},
	}
	sig := &v1.ArgusSignal{
		Category:  "safety.classifier",
		Timestamp: timestamppb.Now(),
		Source:    &v1.Source{AppId: "app-001"},
	}
	ok, err := engine.Tier3Matches(context.Background(), rule, sig, &fakeTemporalStore{count: 5})
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestTier3NoMatch(t *testing.T) {
	rule := engine.Rule{
		ID: "t3-1", Name: "Burst", Tier: 3, Severity: 4,
		Conditions: engine.Conditions{
			Temporal: &engine.TemporalCond{CountGte: 5, WindowSeconds: 300},
		},
		Action: engine.Action{Title: "Burst Alert"},
	}
	sig := &v1.ArgusSignal{
		Category:  "safety.classifier",
		Timestamp: timestamppb.Now(),
		Source:    &v1.Source{AppId: "app-001"},
	}
	ok, err := engine.Tier3Matches(context.Background(), rule, sig, &fakeTemporalStore{count: 3})
	assert.NoError(t, err)
	assert.False(t, ok)
}
