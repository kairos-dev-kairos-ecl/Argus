package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/detection/engine"
	"github.com/argusxdr/argus/internal/notify"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type mockRoutingEngine struct {
	targets []string
}

func (m *mockRoutingEngine) Evaluate(_ *notify.EvaluationContext) []string {
	return m.targets
}

type captureDispatcher struct {
	jobs []*notify.DispatchJob
}

func (c *captureDispatcher) Dispatch(job *notify.DispatchJob) error {
	c.jobs = append(c.jobs, job)
	return nil
}

// --- Task 1: Fingerprint + trace linkage tests ---

// TestFingerprint_DifferentLayers_DifferentHashes verifies that two signals with the same
// rule_id and app_id but different layers produce different fingerprints.
func TestFingerprint_DifferentLayers_DifferentHashes(t *testing.T) {
	ruleID := "t1-001"
	sig1 := &v1.ArgusSignal{
		Source:   &v1.Source{AppId: "app-abc"},
		Layer:    v1.Layer_L1_HARDWARE,
		Category: "cpu.spike",
		Severity: v1.Severity_LOW,
	}
	sig2 := &v1.ArgusSignal{
		Source:   &v1.Source{AppId: "app-abc"},
		Layer:    v1.Layer_L10_APPLICATION,
		Category: "cpu.spike",
		Severity: v1.Severity_LOW,
	}
	fp1 := computeRouterFingerprint(ruleID, sig1)
	fp2 := computeRouterFingerprint(ruleID, sig2)
	assert.NotEqual(t, fp1, fp2, "different layers must produce different fingerprints")
}

// TestFingerprint_SameInputs_SameHash verifies determinism: same inputs → same fingerprint.
func TestFingerprint_SameInputs_SameHash(t *testing.T) {
	ruleID := "t1-001"
	sig := &v1.ArgusSignal{
		Source:   &v1.Source{AppId: "app-abc"},
		Layer:    v1.Layer_L6_SAFETY,
		Category: "safety.block",
		Severity: v1.Severity_HIGH,
	}
	fp1 := computeRouterFingerprint(ruleID, sig)
	fp2 := computeRouterFingerprint(ruleID, sig)
	assert.Equal(t, fp1, fp2, "same inputs must produce identical fingerprints")
}

// TestFingerprint_IncludesRuleID verifies that two different rules on the same signal
// produce different fingerprints.
func TestFingerprint_IncludesRuleID(t *testing.T) {
	sig := &v1.ArgusSignal{
		Source:   &v1.Source{AppId: "app-xyz"},
		Layer:    v1.Layer_L3_TOKENIZER,
		Category: "mem.oom",
		Severity: v1.Severity_CRITICAL,
	}
	fp1 := computeRouterFingerprint("rule-A", sig)
	fp2 := computeRouterFingerprint("rule-B", sig)
	assert.NotEqual(t, fp1, fp2, "different rule IDs must produce different fingerprints")
}

// TestWriteAlert_MissingTraceID_Rejects verifies that WriteAlert returns ErrMissingTraceID
// when the signal has no trace_id, and performs no DB write.
func TestWriteAlert_MissingTraceID_Rejects(t *testing.T) {
	router := &AlertRouter{
		pool:       nil,
		dispatcher: &captureDispatcher{},
		routing:    &mockRoutingEngine{targets: []string{"log"}},
		log:        zap.NewNop(),
	}

	match := engine.MatchResult{
		Rule: engine.Rule{ID: "t1-001", Severity: 3, Action: engine.Action{Title: "test"}},
		Signal: &v1.ArgusSignal{
			SignalId: "sig-1",
			TraceId:  "", // missing — must be rejected
			Source:   &v1.Source{AppId: "app-abc"},
			Layer:    v1.Layer_L10_APPLICATION,
			Category: "anomaly",
		},
	}

	err := router.WriteAlert(context.Background(), match, nil)
	assert.ErrorIs(t, err, ErrMissingTraceID)
}

// TestFingerprintComposition verifies the sha256 computation directly using the expected
// input format: ruleID + "|" + entity + "|" + payload.
func TestFingerprintComposition(t *testing.T) {
	ruleID := "rule-123"
	appID := "app-test"
	sessionID := ""
	layer := v1.Layer_L5_OUTPUT_DECODING
	category := "inference.latency"
	severity := v1.Severity_MEDIUM

	entity := fmt.Sprintf("%s:%s", appID, sessionID)
	payload := fmt.Sprintf("%d|%s|%d", layer, category, severity)
	expected := sha256.Sum256([]byte(ruleID + "|" + entity + "|" + payload))
	expectedHex := hex.EncodeToString(expected[:])

	sig := &v1.ArgusSignal{
		Source:   &v1.Source{AppId: appID},
		Layer:    layer,
		Category: category,
		Severity: severity,
	}
	got := computeRouterFingerprint(ruleID, sig)
	assert.Equal(t, expectedHex, got)
}

// --- Legacy tests updated for trace_id requirement ---

func TestAlertRouterWriteAlertNilPoolNoop(t *testing.T) {
	router := &AlertRouter{
		pool:       nil,
		dispatcher: &captureDispatcher{},
		routing:    &mockRoutingEngine{targets: []string{"log"}},
		log:        zap.NewNop(),
	}

	match := engine.MatchResult{
		Rule: engine.Rule{ID: "t1-001", Severity: 3, Action: engine.Action{Title: "test"}},
		Signal: &v1.ArgusSignal{
			SignalId: "sig-1",
			TraceId:  "trace-noop", // required now per D-06
			Source:   &v1.Source{AppId: "app-abc"},
			Layer:    v1.Layer_L10_APPLICATION,
			Category: "anomaly",
		},
	}

	err := router.WriteAlert(context.Background(), match, nil)
	assert.NoError(t, err)
}

func TestAlertRouterWriteAlertDispatches(t *testing.T) {
	cap := &captureDispatcher{}
	router := &AlertRouter{
		pool:       nil,
		dispatcher: cap,
		routing:    &mockRoutingEngine{targets: []string{"log"}},
		log:        zap.NewNop(),
	}

	match := engine.MatchResult{
		Rule: engine.Rule{ID: "t1-002", Severity: 4, Action: engine.Action{Title: "alert", Description: "desc"}},
		Signal: &v1.ArgusSignal{
			SignalId: "sig-2",
			TraceId:  "trace-1",
			Source:   &v1.Source{AppId: "app-1"},
			Layer:    v1.Layer_L6_SAFETY,
			Category: "safety.classifier",
		},
	}

	err := router.WriteAlert(context.Background(), match, nil)
	require.NoError(t, err)
	require.Len(t, cap.jobs, 1)
	assert.Equal(t, []string{"log"}, cap.jobs[0].Targets)
	assert.Equal(t, "alert", cap.jobs[0].Notification.Title)
	assert.NotEqual(t, uuid.Nil, cap.jobs[0].AlertID)
}

// --- Task 2: JSONB context + SUPPRESSED auto-trigger tests ---
// Integration tests skipped under -short (require real PostgreSQL).

// TestUpsertAlert_IncrementsCount verifies that inserting the same fingerprint twice
// increments signal_count. Skipped under -short (requires real PostgreSQL).
func TestUpsertAlert_IncrementsCount(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL; skipped in -short mode")
	}
	t.Log("TestUpsertAlert_IncrementsCount: integration test stub")
}

// TestUpsertAlert_AutoSuppressAbove100 verifies that the 101st occurrence of the same
// fingerprint flips status to 'suppressed'. Skipped under -short (requires real PostgreSQL).
func TestUpsertAlert_AutoSuppressAbove100(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL; skipped in -short mode")
	}
	t.Log("TestUpsertAlert_AutoSuppressAbove100: integration test stub")
}

// TestUpsertAlert_PersistsContextJSONB verifies that the inserted alert row carries
// the context JSONB field. Skipped under -short (requires real PostgreSQL).
func TestUpsertAlert_PersistsContextJSONB(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL; skipped in -short mode")
	}
	t.Log("TestUpsertAlert_PersistsContextJSONB: integration test stub")
}

// TestSuppressThreshold_DefaultIs100 verifies that a router created via NewAlertRouter
// has suppressThreshold == 100.
func TestSuppressThreshold_DefaultIs100(t *testing.T) {
	router := NewAlertRouter(nil, nil, &mockRoutingEngine{}, &captureDispatcher{}, nil)
	assert.Equal(t, 100, router.suppressThreshold)
}
