//go:build integration

package storage_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// TestMigration018_AlertsTableStructure verifies that migration 018 creates the
// alerts table with all expected columns, the UNIQUE fingerprint index, and that
// the status CHECK constraint accepts all 4 valid states and rejects invalid ones.
func TestMigration018_AlertsTableStructure(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	defer pool.Close()

	// Apply all migrations up to 018
	ApplyMigrations(t, pool, "up", 18)
	defer ApplyMigrations(t, pool, "down", 18)

	// Assert all 22 expected columns exist
	for _, col := range []string{
		"id", "rule_id", "app_id", "trace_id", "signal_ids", "signal_count",
		"fingerprint", "severity", "layer", "category", "title", "description",
		"status", "context", "kairos_decision", "first_seen_at", "last_seen_at",
		"acknowledged_at", "acknowledged_by", "resolved_at", "incident_id",
		"created_at", "updated_at",
	} {
		var exists bool
		err := pool.QueryRow(context.Background(),
			`SELECT EXISTS(
				SELECT 1 FROM information_schema.columns
				WHERE table_name='alerts' AND column_name=$1
			)`, col).Scan(&exists)
		require.NoError(t, err)
		require.True(t, exists, "column %s missing from alerts table", col)
	}

	// Assert UNIQUE index on fingerprint
	var indexDef string
	err = pool.QueryRow(context.Background(),
		`SELECT indexdef FROM pg_indexes WHERE indexname = 'idx_alerts_fingerprint'`,
	).Scan(&indexDef)
	require.NoError(t, err, "idx_alerts_fingerprint index not found")
	require.Contains(t, indexDef, "UNIQUE", "idx_alerts_fingerprint should be UNIQUE")

	// Assert status CHECK constraint allows all 4 valid states
	for _, status := range []string{"open", "acknowledged", "resolved", "suppressed"} {
		_, err := pool.Exec(context.Background(),
			`INSERT INTO alerts (rule_id, fingerprint, severity, status)
			 VALUES ('test-rule', 'fp-'||$1, 3, $1)`,
			status)
		require.NoError(t, err, "valid status %q was rejected by CHECK constraint", status)
	}

	// Assert invalid status is rejected
	_, err = pool.Exec(context.Background(),
		`INSERT INTO alerts (rule_id, fingerprint, severity, status)
		 VALUES ('test-rule', 'fp-bad', 3, 'closed')`)
	require.Error(t, err, "invalid status 'closed' should have been rejected")
}
