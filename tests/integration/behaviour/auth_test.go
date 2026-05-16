package behaviour_integration

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuth_NoToken_Returns401 verifies that every Phase 7 endpoint returns 401
// when called without an Authorization header.
func TestAuth_NoToken_Returns401(t *testing.T) {
	skipIfNoIntegration(t)

	for _, path := range phase7Paths {
		path := path // capture for parallel sub-tests
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			resp := doGET(t, path, "")
			defer resp.Body.Close()
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
				"path %s should return 401 without a token", path)
		})
	}
}

// TestAuth_ViewerToken_Returns403 verifies that a JWT with only the viewer
// role is rejected with 403 on all Phase 7 endpoints.
func TestAuth_ViewerToken_Returns403(t *testing.T) {
	skipIfNoIntegration(t)
	if token := os.Getenv("ARGUS_TEST_JWT_VIEWER"); token == "" {
		t.Skip("ARGUS_TEST_JWT_VIEWER unset; skipping viewer-role test")
	}

	for _, path := range phase7Paths {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			resp := doGET(t, path, "ARGUS_TEST_JWT_VIEWER")
			defer resp.Body.Close()
			assert.Equal(t, http.StatusForbidden, resp.StatusCode,
				"path %s should return 403 for viewer-role token", path)
		})
	}
}

// TestSchema_SkipIndexesPresent verifies that the two ClickHouse skip indexes
// required by REQ-P7-01 are present in the deployed schema.
func TestSchema_SkipIndexesPresent(t *testing.T) {
	skipIfNoIntegration(t)
	ch := newCH(t)
	defer ch.Close()

	rows, err := ch.Query(context.Background(),
		`SELECT name FROM system.data_skipping_indices
		 WHERE table = 'signals'
		   AND name IN ('idx_skip_session_id', 'idx_skip_conversation_id')`)
	require.NoError(t, err)
	defer rows.Close()

	var found []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		found = append(found, name)
	}
	require.NoError(t, rows.Err())

	assert.Contains(t, found, "idx_skip_session_id",
		"ClickHouse skip index idx_skip_session_id must be present (REQ-P7-01)")
	assert.Contains(t, found, "idx_skip_conversation_id",
		"ClickHouse skip index idx_skip_conversation_id must be present (REQ-P7-01)")
}

// TestSchema_SessionBaselineTablePresent verifies that the session_baseline_profiles
// table created by migrations/011_session_baseline.up.sql is present in PostgreSQL.
// This validates REQ-P7-02.
func TestSchema_SessionBaselineTablePresent(t *testing.T) {
	skipIfNoIntegration(t)
	pg := newPG(t)
	defer pg.Close()

	var count int
	err := pg.QueryRow(context.Background(),
		`SELECT count(*) FROM information_schema.tables
		 WHERE table_name = 'session_baseline_profiles'`).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count,
		"session_baseline_profiles table must be present (REQ-P7-02 — migration 011)")
}
