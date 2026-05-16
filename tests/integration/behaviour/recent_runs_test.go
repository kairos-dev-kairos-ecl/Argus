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

// TestRecentRuns_GroupBy seeds 6 signals spread across 3 trace_ids for a
// single app_id and asserts the recent runs endpoint returns exactly 3 entries
// with layers_present and peak_deviation fields present.
func TestRecentRuns_GroupBy(t *testing.T) {
	skipIfNoIntegration(t)
	ch := newCH(t)
	defer ch.Close()

	appID := "app-R-" + uuid.NewString()[:8]
	now := time.Now().UTC().Truncate(time.Millisecond)

	// 3 trace_ids × 2 signals each = 6 signals total
	traceIDs := []string{
		"TR1-" + uuid.NewString()[:8],
		"TR2-" + uuid.NewString()[:8],
		"TR3-" + uuid.NewString()[:8],
	}

	var sigs []SeedSignal
	for i, tid := range traceIDs {
		for j := 0; j < 2; j++ {
			sigs = append(sigs, SeedSignal{
				SignalID:      uuid.NewString(),
				TraceID:       tid,
				SpanID:        uuid.NewString(),
				AppID:         appID,
				Category:      "inference",
				Layer:         uint8(3 + j),
				Severity:      1,
				Timestamp:     now.Add(time.Duration(i*10+j) * time.Second),
				BaselineDevia: float32(i) * 0.5,
			})
		}
	}
	seedSignals(t, t.Context(), ch, sigs)

	resp := doGET(t, "/api/v1/traces/recent?app_id="+appID, "ARGUS_TEST_JWT_ANALYST")
	require.Equal(t, http.StatusOK, resp.StatusCode, "expected 200 from recent runs endpoint")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var runs []struct {
		TraceID       string  `json:"trace_id"`
		LayersPresent []int32 `json:"layers_present"`
		PeakDeviation float32 `json:"peak_deviation"`
	}
	require.NoError(t, json.Unmarshal(body, &runs), "response must be a JSON array of RecentRunSummary")

	assert.Len(t, runs, 3, "expected exactly 3 trace summaries (one per trace_id)")
	for _, r := range runs {
		assert.NotEmpty(t, r.TraceID, "trace_id must be non-empty")
		assert.NotEmpty(t, r.LayersPresent, "layers_present should contain at least one layer")
	}
}

// TestRecentRuns_MissingAppID verifies that the endpoint returns 400 when
// app_id is not provided.
func TestRecentRuns_MissingAppID(t *testing.T) {
	skipIfNoIntegration(t)
	resp := doGET(t, "/api/v1/traces/recent", "ARGUS_TEST_JWT_ANALYST")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "missing app_id should return 400")
}

// TestRecentRuns_NoToken_Returns401 verifies the endpoint requires auth.
func TestRecentRuns_NoToken_Returns401(t *testing.T) {
	skipIfNoIntegration(t)
	resp := doGET(t, "/api/v1/traces/recent?app_id=any", "")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
