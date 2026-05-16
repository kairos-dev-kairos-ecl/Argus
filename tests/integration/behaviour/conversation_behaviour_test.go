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

// TestConversationBehaviour_DriftScore seeds signals for a conversation with a
// known layer sequence and a matching session baseline profile, then asserts
// that drift_score is 0.0 (identical sequences produce zero drift).
func TestConversationBehaviour_DriftScore(t *testing.T) {
	skipIfNoIntegration(t)
	ch := newCH(t)
	defer ch.Close()
	pg := newPG(t)
	defer pg.Close()

	appID := "app-T-" + uuid.NewString()[:8]
	convID := "C1-" + uuid.NewString()[:8]
	now := time.Now().UTC().Truncate(time.Millisecond)

	layers := []uint8{3, 5, 7}
	sigs := make([]SeedSignal, len(layers))
	for i, l := range layers {
		sigs[i] = SeedSignal{
			SignalID:  uuid.NewString(),
			TraceID:   uuid.NewString(),
			SpanID:    uuid.NewString(),
			AppID:     appID,
			Category:  "inference",
			Layer:     l,
			Severity:  1,
			Timestamp: now.Add(time.Duration(i) * time.Second),
			ConvID:    convID,
		}
	}
	seedSignals(t, t.Context(), ch, sigs)

	// Seed a matching baseline: identical layer sequence → drift == 0.0
	seedSessionProfile(t, t.Context(), pg, appID, []int32{3, 5, 7})

	resp := doGET(t, "/api/v1/conversations/"+convID+"/behaviour", "ARGUS_TEST_JWT_ANALYST")
	require.Equal(t, http.StatusOK, resp.StatusCode, "expected 200 from conversation behaviour endpoint")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var result struct {
		DriftScore *float64 `json:"drift_score"`
		AppID      string   `json:"app_id"`
	}
	require.NoError(t, json.Unmarshal(body, &result), "response must be valid conversation behaviour JSON")

	require.NotNil(t, result.DriftScore, "drift_score must be non-null when baseline exists")
	assert.InDelta(t, 0.0, *result.DriftScore, 0.0001,
		"drift_score should be 0.0 when actual sequence matches baseline")
	assert.Equal(t, appID, result.AppID, "app_id should match the seeded app")
}

// TestConversationBehaviour_NoBaseline seeds signals with no session baseline
// profile and asserts that drift_score is null (missing baseline is acceptable).
func TestConversationBehaviour_NoBaseline(t *testing.T) {
	skipIfNoIntegration(t)
	ch := newCH(t)
	defer ch.Close()

	appID := "app-nobase-" + uuid.NewString()[:8]
	convID := "C2-" + uuid.NewString()[:8]
	now := time.Now().UTC().Truncate(time.Millisecond)

	sigs := []SeedSignal{
		{
			SignalID:  uuid.NewString(),
			TraceID:   uuid.NewString(),
			SpanID:    uuid.NewString(),
			AppID:     appID,
			Category:  "inference",
			Layer:     5,
			Severity:  1,
			Timestamp: now,
			ConvID:    convID,
		},
	}
	seedSignals(t, t.Context(), ch, sigs)
	// No seedSessionProfile call — baseline intentionally absent.

	resp := doGET(t, "/api/v1/conversations/"+convID+"/behaviour", "ARGUS_TEST_JWT_ANALYST")
	require.Equal(t, http.StatusOK, resp.StatusCode, "expected 200 even with no baseline")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var result struct {
		DriftScore *float64 `json:"drift_score"`
	}
	require.NoError(t, json.Unmarshal(body, &result), "response must be valid JSON")
	assert.Nil(t, result.DriftScore, "drift_score should be null when no baseline exists")
}

// TestConversationBehaviour_NoToken_Returns401 verifies the endpoint requires auth.
func TestConversationBehaviour_NoToken_Returns401(t *testing.T) {
	skipIfNoIntegration(t)
	resp := doGET(t, "/api/v1/conversations/any-conv-id/behaviour", "")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
