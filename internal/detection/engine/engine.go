package engine

import (
	"context"
	"fmt"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
)

// MatchResult describes one rule hit for a signal.
type MatchResult struct {
	Rule   Rule
	Tier   int
	Signal *v1.ArgusSignal
	Reason string
}

// DetectionEngine evaluates signals against enabled rules.
type DetectionEngine struct {
	store    *RuleStore
	temporal TemporalStore
}

// New creates a detection engine.
func New(store *RuleStore, temporal TemporalStore) *DetectionEngine {
	if store == nil {
		store = NewRuleStore()
	}
	return &DetectionEngine{
		store:    store,
		temporal: temporal,
	}
}

// Evaluate evaluates one signal against all enabled rules.
func (e *DetectionEngine) Evaluate(ctx context.Context, sig *v1.ArgusSignal) ([]MatchResult, error) {
	if sig == nil {
		return nil, nil
	}
	rules := e.store.Enabled()
	out := make([]MatchResult, 0, len(rules))

	for _, r := range rules {
		switch r.Tier {
		case 1:
			if Tier1Matches(r, sig) {
				out = append(out, MatchResult{Rule: r, Tier: 1, Signal: sig, Reason: "tier1"})
			}
		case 2:
			if Tier2Matches(r, sig) {
				out = append(out, MatchResult{Rule: r, Tier: 2, Signal: sig, Reason: "tier2"})
			}
		case 3:
			ok, err := Tier3Matches(ctx, r, sig, e.temporal)
			if err != nil {
				return out, fmt.Errorf("tier3 rule %q: %w", r.ID, err)
			}
			if ok {
				out = append(out, MatchResult{Rule: r, Tier: 3, Signal: sig, Reason: "tier3"})
			}
		}
	}
	return out, nil
}
