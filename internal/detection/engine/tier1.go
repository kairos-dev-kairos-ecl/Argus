package engine

import (
	"strings"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
)

// Tier1Matches evaluates deterministic field-based rule conditions.
func Tier1Matches(rule Rule, sig *v1.ArgusSignal) bool {
	if sig == nil {
		return false
	}
	if layer := strings.TrimSpace(rule.Conditions.Layer); layer != "" && sig.Layer.String() != layer {
		return false
	}
	if prefix := strings.TrimSpace(rule.Conditions.CategoryPrefix); prefix != "" && !strings.HasPrefix(sig.Category, prefix) {
		return false
	}
	if rule.Conditions.SeverityGte > 0 && int(sig.Severity) < rule.Conditions.SeverityGte {
		return false
	}
	return true
}
