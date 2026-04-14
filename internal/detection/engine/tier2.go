package engine

import v1 "github.com/argusxdr/argus/gen/go/argus/v1"

// Tier2Matches evaluates baseline deviation conditions.
func Tier2Matches(rule Rule, sig *v1.ArgusSignal) bool {
	if sig == nil || sig.Enrichment == nil || sig.Enrichment.BaselineDeviation == nil {
		return false
	}
	threshold := rule.Conditions.BaselineDeviationGte
	if threshold <= 0 {
		return false
	}
	return *sig.Enrichment.BaselineDeviation >= threshold
}
