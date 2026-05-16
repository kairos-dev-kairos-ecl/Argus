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

// TestAlertChain_ReturnsGraph seeds two signals with a parent-child span
// relationship, creates an alert referencing the first signal, then asserts
// that the alert chain endpoint returns a RunGraph containing both nodes.
func TestAlertChain_ReturnsGraph(t *testing.T) {
	skipIfNoIntegration(t)
	ch := newCH(t)
	defer ch.Close()
	pg := newPG(t)
	defer pg.Close()

	traceID := "TA-" + uuid.NewString()[:8]
	now := time.Now().UTC().Truncate(time.Millisecond)

	sigX := SeedSignal{
		SignalID:  uuid.NewString(),
		TraceID:   traceID,
		SpanID:    "x",
		AppID:     "app-alert-test",
		Category:  "inference",
		Layer:     5,
		Severity:  2,
		Timestamp: now,
	}
	sigY := SeedSignal{
		SignalID:     uuid.NewString(),
		TraceID:      traceID,
		SpanID:       "y",
		ParentSpanID: "x",
		AppID:        "app-alert-test",
		Category:     "retrieval",
		Layer:        7,
		Severity:     2,
		Timestamp:    now.Add(time.Second),
	}
	seedSignals(t, t.Context(), ch, []SeedSignal{sigX, sigY})

	alertID := seedAlert(t, t.Context(), pg, []string{sigX.SignalID})

	resp := doGET(t, "/api/v1/alerts/"+alertID+"/chain", "ARGUS_TEST_JWT_ANALYST")
	require.Equal(t, http.StatusOK, resp.StatusCode, "expected 200 from alert chain endpoint")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var graph struct {
		Nodes []struct {
			SpanID string `json:"span_id"`
		} `json:"nodes"`
	}
	require.NoError(t, json.Unmarshal(body, &graph), "response must be a valid RunGraph JSON")

	// Both signals share the trace, so the full graph includes both nodes.
	spanIDs := make(map[string]bool)
	for _, n := range graph.Nodes {
		spanIDs[n.SpanID] = true
	}
	assert.True(t, spanIDs["x"], "graph should contain span x")
	assert.True(t, spanIDs["y"], "graph should contain span y (full trace returned)")
}

// TestAlertChain_NoToken_Returns401 verifies the endpoint requires auth.
func TestAlertChain_NoToken_Returns401(t *testing.T) {
	skipIfNoIntegration(t)
	resp := doGET(t, "/api/v1/alerts/00000000-0000-0000-0000-000000000000/chain", "")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
