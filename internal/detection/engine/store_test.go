package engine_test

import (
	"sync"
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

func TestRuleStore_ReplaceAll_Atomic(t *testing.T) {
	s := engine.NewRuleStore()

	makeRule := func(id string) engine.Rule {
		return engine.Rule{
			ID:       id,
			Name:     "Rule " + id,
			Tier:     1,
			Enabled:  true,
			Severity: 3,
			Action:   engine.Action{Title: "Alert"},
		}
	}

	// Seed 3 rules.
	s.Add(makeRule("a"))
	s.Add(makeRule("b"))
	s.Add(makeRule("c"))

	newRules := []engine.Rule{makeRule("x"), makeRule("y")}

	var wg sync.WaitGroup
	badCount := 0
	var mu sync.Mutex

	// 100 concurrent readers checking count during ReplaceAll.
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			count := s.Count()
			if count != 3 && count != 2 {
				mu.Lock()
				badCount++
				mu.Unlock()
			}
		}()
	}

	// Swap rules while readers are running.
	s.ReplaceAll(newRules)
	wg.Wait()

	assert.Equal(t, 0, badCount, "readers saw partial state during ReplaceAll")
	assert.Equal(t, 2, s.Count())
}
