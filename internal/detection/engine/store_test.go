package engine_test

import (
	"testing"

	"github.com/argusxdr/argus/internal/detection/engine"
	"github.com/stretchr/testify/assert"
)

func TestRuleStoreCRUD(t *testing.T) {
	s := engine.NewRuleStore()
	assert.Equal(t, 0, s.Count())

	r := engine.Rule{
		ID:       "r1",
		Name:     "Rule 1",
		Tier:     1,
		Enabled:  true,
		Severity: 3,
		Action:   engine.Action{Title: "Alert"},
	}
	s.Add(r)
	assert.Equal(t, 1, s.Count())

	got, ok := s.Get("r1")
	assert.True(t, ok)
	assert.Equal(t, "Rule 1", got.Name)

	all := s.All()
	assert.Len(t, all, 1)
	assert.Len(t, s.Enabled(), 1)

	s.Remove("r1")
	assert.Equal(t, 0, s.Count())
}
