package trace

import (
	"testing"
	"time"
)

// TestTypesCompile ensures all exported types in graph.go, query.go, and timeline.go
// compile correctly. The real graph-assembly tests are in TestAssembleGraph below.
func TestTypesCompile(t *testing.T) {
	_ = RunGraph{}
	_ = RunNode{}
	_ = RunEdge{}
	_ = RunMeta{}
	_ = EdgeType("")
	_ = EdgeParentChild
	_ = EdgeTemporal
	_ = AnomalyDeviationThreshold
	_ = SelectRunSignalsSQL
	_ = SelectSessionSignalsSQLBySessionID
	_ = SelectSessionSignalsSQLByConversationID
	_ = SessionTimeline{}
	_ = TimelineEvent{}
	_ = SessionAggregates{}
}

// TestAssembleGraph tests the in-memory graph assembly logic with three nodes:
//   - root: span "A", no parent
//   - child: span "B", parent "A"  (parent_child edge expected)
//   - orphan: span "C", parent "MISSING" (not in set → IsOrphan=true, temporal edge expected)
func TestAssembleGraph(t *testing.T) {
	t0 := time.Now()
	nodes := []*RunNode{
		{SignalID: "sig1", SpanID: "A", ParentSpanID: "", Layer: 3, BaselineDeviation: 0.5, Timestamp: t0},
		{SignalID: "sig2", SpanID: "B", ParentSpanID: "A", Layer: 5, BaselineDeviation: 2.5, Timestamp: t0.Add(time.Millisecond)},
		{SignalID: "sig3", SpanID: "C", ParentSpanID: "MISSING", Layer: 7, BaselineDeviation: 1.2, Timestamp: t0.Add(2 * time.Millisecond)},
	}

	graph := assembleGraph("trace-001", nodes)

	// 3 nodes total
	if len(graph.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(graph.Nodes))
	}

	// orphan must be flagged
	var orphan *RunNode
	for _, n := range graph.Nodes {
		if n.SpanID == "C" {
			orphan = n
		}
	}
	if orphan == nil {
		t.Fatal("node C not found in graph")
	}
	if !orphan.IsOrphan {
		t.Error("expected node C to be flagged IsOrphan=true")
	}

	// exactly 1 parent_child edge (B→A)
	parentChildEdges := 0
	for _, e := range graph.Edges {
		if e.Type == EdgeParentChild {
			parentChildEdges++
			if e.FromSpanID != "A" || e.ToSpanID != "B" {
				t.Errorf("unexpected parent_child edge: %v→%v", e.FromSpanID, e.ToSpanID)
			}
		}
	}
	if parentChildEdges != 1 {
		t.Errorf("expected 1 parent_child edge, got %d", parentChildEdges)
	}

	// PeakDeviation == max(0.5, 2.5, 1.2) == 2.5
	if graph.Meta.PeakDeviation != 2.5 {
		t.Errorf("expected PeakDeviation=2.5, got %f", graph.Meta.PeakDeviation)
	}

	// OrphanCount == 1
	if graph.Meta.OrphanCount != 1 {
		t.Errorf("expected OrphanCount=1, got %d", graph.Meta.OrphanCount)
	}

	// SignalCount == 3
	if graph.Meta.SignalCount != 3 {
		t.Errorf("expected SignalCount=3, got %d", graph.Meta.SignalCount)
	}

	// node B (deviation 2.5 > 2.0) must be anomaly
	for _, n := range graph.Nodes {
		if n.SpanID == "B" && !n.IsAnomaly {
			t.Error("expected node B (deviation=2.5) to be IsAnomaly=true")
		}
		if n.SpanID == "A" && n.IsAnomaly {
			t.Error("expected node A (deviation=0.5) to be IsAnomaly=false")
		}
	}

	// LayersPresent must be sorted: [3, 5, 7]
	want := []int32{3, 5, 7}
	if len(graph.Meta.LayersPresent) != len(want) {
		t.Fatalf("expected LayersPresent=%v, got %v", want, graph.Meta.LayersPresent)
	}
	for i, v := range want {
		if graph.Meta.LayersPresent[i] != v {
			t.Errorf("LayersPresent[%d]: expected %d, got %d", i, v, graph.Meta.LayersPresent[i])
		}
	}
}
