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

// TestSessionTimeline_Ordered seeds 4 signals sharing a session_id across
// different layers and asserts the timeline endpoint returns them ordered by
// timestamp with the correct layer_activation_sequence.
func TestSessionTimeline_Ordered(t *testing.T) {
	skipIfNoIntegration(t)
	ch := newCH(t)
	defer ch.Close()

	sessionID := "S1-" + uuid.NewString()[:8]
	now := time.Now().UTC().Truncate(time.Millisecond)

	// Layers visited in order: 3, 5, 3, 7 — deduplicated activation sequence: [3, 5, 7]
	layerSeq := []uint8{3, 5, 3, 7}
	sigs := make([]SeedSignal, len(layerSeq))
	for i, l := range layerSeq {
		sigs[i] = SeedSignal{
			SignalID:  uuid.NewString(),
			TraceID:   uuid.NewString(),
			SpanID:    uuid.NewString(),
			AppID:     "app-timeline-test",
			Category:  "inference",
			Layer:     l,
			Severity:  1,
			Timestamp: now.Add(time.Duration(i) * time.Second),
			SessionID: sessionID,
		}
	}
	seedSignals(t, t.Context(), ch, sigs)

	resp := doGET(t, "/api/v1/sessions/"+sessionID+"/timeline", "ARGUS_TEST_JWT_ANALYST")
	require.Equal(t, http.StatusOK, resp.StatusCode, "expected 200 from session timeline endpoint")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var tl struct {
		Aggregates struct {
			TotalSignals            int     `json:"total_signals"`
			LayerActivationSequence []int32 `json:"layer_activation_sequence"`
		} `json:"aggregates"`
	}
	require.NoError(t, json.Unmarshal(body, &tl), "response must be valid SessionTimeline JSON")

	assert.Equal(t, 4, tl.Aggregates.TotalSignals, "expected 4 total signals")
	// layer_activation_sequence is deduplicated in encounter order: [3, 5, 7]
	assert.Equal(t, []int32{3, 5, 7}, tl.Aggregates.LayerActivationSequence,
		"layer_activation_sequence should be deduplicated in encounter order")
}

// TestSessionTimeline_NoToken_Returns401 verifies the endpoint requires auth.
func TestSessionTimeline_NoToken_Returns401(t *testing.T) {
	skipIfNoIntegration(t)
	resp := doGET(t, "/api/v1/sessions/any-session-id/timeline", "")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
