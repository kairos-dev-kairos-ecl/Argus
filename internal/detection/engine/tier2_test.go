package engine_test

import (
	"testing"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/detection/engine"
	"github.com/stretchr/testify/assert"
)

func TestTier2Matches(t *testing.T) {
	bd := float32(3.2)
	sig := &v1.ArgusSignal{
		Enrichment: &v1.Enrichment{
			BaselineDeviation: &bd,
		},
	}
	r := engine.Rule{
		ID: "r2", Name: "Tier2", Tier: 2, Severity: 3,
		Conditions: engine.Conditions{BaselineDeviationGte: 2.5},
		Action:     engine.Action{Title: "Deviation"},
	}
	assert.True(t, engine.Tier2Matches(r, sig))
}

func TestTier2NoMatch(t *testing.T) {
	bd := float32(0.5)
	sig := &v1.ArgusSignal{
		Enrichment: &v1.Enrichment{
			BaselineDeviation: &bd,
		},
	}
	r := engine.Rule{
		ID: "r2", Name: "Tier2", Tier: 2, Severity: 3,
		Conditions: engine.Conditions{BaselineDeviationGte: 2.5},
		Action:     engine.Action{Title: "Deviation"},
	}
	assert.False(t, engine.Tier2Matches(r, sig))
}
