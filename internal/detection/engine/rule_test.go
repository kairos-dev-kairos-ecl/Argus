package engine_test

import (
	"testing"

	"github.com/argusxdr/argus/internal/detection/engine"
	"github.com/stretchr/testify/assert"
)

func TestRuleValidation(t *testing.T) {
	r := engine.Rule{
		ID: "argus-t1-001", Name: "Test", Tier: 1, Enabled: true, Severity: 3,
		Conditions: engine.Conditions{Layer: "L6_SAFETY"},
		Action:     engine.Action{Title: "Test Alert"},
	}
	assert.NoError(t, r.Validate())

	bad := engine.Rule{ID: "", Tier: 1}
	assert.Error(t, bad.Validate())

	badTier := engine.Rule{
		ID: "x", Name: "x", Tier: 4, Severity: 3,
		Action: engine.Action{Title: "x"},
	}
	assert.Error(t, badTier.Validate())
}
