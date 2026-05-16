package trace

import (
	"testing"
	"time"
)

// TestAssembleTimeline_LayerActivation verifies that LayerActivationSequence records
// the FIRST occurrence of each layer in timestamp order, with no duplicates.
//
// Input: events [t=0 L3, t=1 L5, t=2 L3, t=3 L7]
// Expected: LayerActivationSequence == [3, 5, 7], TotalSignals == 4
func TestAssembleTimeline_LayerActivation(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	events := []*TimelineEvent{
		{SignalID: "e1", Timestamp: t0, Layer: 3, LayerLabel: "L3", BaselineDeviation: 0.5},
		{SignalID: "e2", Timestamp: t0.Add(time.Second), Layer: 5, LayerLabel: "L5", BaselineDeviation: 2.5, IsAnomaly: true},
		{SignalID: "e3", Timestamp: t0.Add(2 * time.Second), Layer: 3, LayerLabel: "L3", BaselineDeviation: 0.8},
		{SignalID: "e4", Timestamp: t0.Add(3 * time.Second), Layer: 7, LayerLabel: "L7", BaselineDeviation: 1.5},
	}

	st := assembleTimeline("session", "sess-001", events)

	// LayerActivationSequence must be [3, 5, 7] — first-seen temporal order, no duplicates.
	wantSeq := []int32{3, 5, 7}
	if len(st.Aggregates.LayerActivationSequence) != len(wantSeq) {
		t.Fatalf("LayerActivationSequence length: expected %d, got %d (%v)",
			len(wantSeq), len(st.Aggregates.LayerActivationSequence), st.Aggregates.LayerActivationSequence)
	}
	for i, v := range wantSeq {
		if st.Aggregates.LayerActivationSequence[i] != v {
			t.Errorf("LayerActivationSequence[%d]: expected %d, got %d",
				i, v, st.Aggregates.LayerActivationSequence[i])
		}
	}

	// TotalSignals == 4
	if st.Aggregates.TotalSignals != 4 {
		t.Errorf("TotalSignals: expected 4, got %d", st.Aggregates.TotalSignals)
	}

	// PeakDeviation == max(0.5, 2.5, 0.8, 1.5) == 2.5
	if st.Aggregates.PeakDeviation != 2.5 {
		t.Errorf("PeakDeviation: expected 2.5, got %f", st.Aggregates.PeakDeviation)
	}

	// AnomalyCount == 1 (only e2 has IsAnomaly=true in our fixture)
	if st.Aggregates.AnomalyCount != 1 {
		t.Errorf("AnomalyCount: expected 1, got %d", st.Aggregates.AnomalyCount)
	}

	// DurationMS == 3000ms (t0+3s - t0)
	if st.Aggregates.DurationMS != 3000 {
		t.Errorf("DurationMS: expected 3000, got %d", st.Aggregates.DurationMS)
	}

	// StartTime and EndTime
	if !st.StartTime.Equal(t0) {
		t.Errorf("StartTime: expected %v, got %v", t0, st.StartTime)
	}
	if !st.EndTime.Equal(t0.Add(3 * time.Second)) {
		t.Errorf("EndTime: expected %v, got %v", t0.Add(3*time.Second), st.EndTime)
	}

	// ByLayer must have 3 entries in first-seen order: L3, L5, L7
	if len(st.ByLayer) != 3 {
		t.Fatalf("ByLayer length: expected 3, got %d", len(st.ByLayer))
	}
	if st.ByLayer[0].Layer != 3 || st.ByLayer[0].Count != 2 {
		t.Errorf("ByLayer[0]: expected Layer=3,Count=2; got Layer=%d,Count=%d", st.ByLayer[0].Layer, st.ByLayer[0].Count)
	}
	if st.ByLayer[1].Layer != 5 || st.ByLayer[1].Count != 1 {
		t.Errorf("ByLayer[1]: expected Layer=5,Count=1; got Layer=%d,Count=%d", st.ByLayer[1].Layer, st.ByLayer[1].Count)
	}
	if st.ByLayer[2].Layer != 7 || st.ByLayer[2].Count != 1 {
		t.Errorf("ByLayer[2]: expected Layer=7,Count=1; got Layer=%d,Count=%d", st.ByLayer[2].Layer, st.ByLayer[2].Count)
	}

	// ScopeKind and ScopeID
	if st.ScopeKind != "session" {
		t.Errorf("ScopeKind: expected 'session', got %q", st.ScopeKind)
	}
	if st.ScopeID != "sess-001" {
		t.Errorf("ScopeID: expected 'sess-001', got %q", st.ScopeID)
	}
}

// TestAssembleTimeline_Empty verifies that an empty event list produces a valid zero-value timeline.
func TestAssembleTimeline_Empty(t *testing.T) {
	st := assembleTimeline("conversation", "conv-empty", nil)
	if st.Aggregates.TotalSignals != 0 {
		t.Errorf("expected TotalSignals=0, got %d", st.Aggregates.TotalSignals)
	}
	if len(st.Events) != 0 {
		t.Errorf("expected empty Events, got %d", len(st.Events))
	}
}
