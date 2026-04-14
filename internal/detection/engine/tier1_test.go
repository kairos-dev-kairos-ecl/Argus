package engine_test

import (
	"testing"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/detection/engine"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func makeSignal(layer v1.Layer, category string, severity v1.Severity) *v1.ArgusSignal {
	return &v1.ArgusSignal{
		SignalId:   "sig-001",
		TraceId:    "trace-001",
		Layer:      layer,
		Category:   category,
		Severity:   severity,
		Timestamp:  timestamppb.Now(),
		IngestedAt: timestamppb.Now(),
		Source:     &v1.Source{AppId: "app-001"},
	}
}

func TestTier1Matches(t *testing.T) {
	rule := engine.Rule{
		ID: "argus-t1-001", Name: "Test", Tier: 1, Enabled: true, Severity: 3,
		Conditions: engine.Conditions{
			Layer:          "L6_SAFETY",
			CategoryPrefix: "safety.",
			SeverityGte:    3,
		},
		Action: engine.Action{Title: "Prompt Injection"},
	}
	sig := makeSignal(v1.Layer_L6_SAFETY, "safety.classifier", v1.Severity_HIGH)
	assert.True(t, engine.Tier1Matches(rule, sig))
}

func TestTier1NoMatchWrongLayer(t *testing.T) {
	rule := engine.Rule{
		ID: "r1", Name: "r", Tier: 1, Severity: 3,
		Conditions: engine.Conditions{Layer: "L6_SAFETY"},
		Action:     engine.Action{Title: "x"},
	}
	sig := makeSignal(v1.Layer_L5_OUTPUT_DECODING, "safety.classifier", v1.Severity_HIGH)
	assert.False(t, engine.Tier1Matches(rule, sig))
}

func TestTier1NoMatchLowSeverity(t *testing.T) {
	rule := engine.Rule{
		ID: "r1", Name: "r", Tier: 1, Severity: 3,
		Conditions: engine.Conditions{Layer: "L6_SAFETY", SeverityGte: 4},
		Action:     engine.Action{Title: "x"},
	}
	sig := makeSignal(v1.Layer_L6_SAFETY, "safety.classifier", v1.Severity_MEDIUM)
	assert.False(t, engine.Tier1Matches(rule, sig))
}

func TestTier1EmptyConditionsAlwaysMatches(t *testing.T) {
	rule := engine.Rule{
		ID: "r1", Name: "r", Tier: 1, Severity: 1,
		Conditions: engine.Conditions{},
		Action:     engine.Action{Title: "x"},
	}
	sig := makeSignal(v1.Layer_L1_HARDWARE, "hw.event", v1.Severity_LOW)
	assert.True(t, engine.Tier1Matches(rule, sig))
}
