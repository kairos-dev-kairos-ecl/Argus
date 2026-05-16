package behaviour_integration

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTraceGraph_ParentChildEdges seeds three signals with a shared trace_id
// forming a linear parent→child→grandchild chain and asserts that the graph
// endpoint returns 3 nodes and 2 parent_child edges.
func TestTraceGraph_ParentChildEdges(t *testing.T) {
	skipIfNoIntegration(t)
	ch := newCH(t)
	defer ch.Close()

	traceID := "T1-" + uuid.NewString()[:8]
	now := time.Now().UTC().Truncate(time.Millisecond)

	sigs := []SeedSignal{
		{
			SignalID:  uuid.NewString(),
			TraceID:   traceID,
			SpanID:    "s1",
			AppID:     "app-graph-test",
			Category:  "inference",
			Layer:     5,
			Severity:  1,
			Timestamp: now,
		},
		{
			SignalID:     uuid.NewString(),
			TraceID:      traceID,
			SpanID:       "s2",
			ParentSpanID: "s1",
			AppID:        "app-graph-test",
			Category:     "inference",
			Layer:        3,
			Severity:     1,
			Timestamp:    now.Add(time.Second),
		},
		{
			SignalID:     uuid.NewString(),
			TraceID:      traceID,
			SpanID:       "s3",
			ParentSpanID: "s2",
			AppID:        "app-graph-test",
			Category:     "retrieval",
			Layer:        7,
			Severity:     2,
			Timestamp:    now.Add(2 * time.Second),
		},
	}
	seedSignals(t, t.Context(), ch, sigs)

	resp := doGET(t, "/api/v1/traces/"+traceID+"/graph", "ARGUS_TEST_JWT_ANALYST")
	require.Equal(t, http.StatusOK, resp.StatusCode, "expected 200 from trace graph endpoint")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var graph struct {
		Nodes []struct {
			SpanID string `json:"span_id"`
		} `json:"nodes"`
		Edges []struct {
			FromSpanID string `json:"from_span_id"`
			ToSpanID   string `json:"to_span_id"`
			Type       string `json:"type"`
		} `json:"edges"`
	}
	require.NoError(t, json.Unmarshal(body, &graph), "response must be valid JSON RunGraph")

	assert.Len(t, graph.Nodes, 3, "expected 3 nodes (one per signal)")
	assert.Len(t, graph.Edges, 2, "expected 2 parent_child edges")

	for _, e := range graph.Edges {
		assert.Equal(t, "parent_child", e.Type, "all seeded edges should be parent_child")
	}

	// Verify the specific edges are present.
	edgeSet := make(map[[2]string]bool)
	for _, e := range graph.Edges {
		edgeSet[[2]string{e.FromSpanID, e.ToSpanID}] = true
	}
	assert.True(t, edgeSet[[2]string{"s1", "s2"}], "expected edge s1→s2")
	assert.True(t, edgeSet[[2]string{"s2", "s3"}], "expected edge s2→s3")
}

// TestTraceGraph_NoToken_Returns401 verifies that the endpoint returns 401
// when called without an Authorization header.
func TestTraceGraph_NoToken_Returns401(t *testing.T) {
	skipIfNoIntegration(t)
	resp := doGET(t, "/api/v1/traces/any-trace-id/graph", "")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
