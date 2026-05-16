// Package behaviour_integration holds end-to-end integration tests for the
// Phase 7 behavioural traceability HTTP endpoints.
//
// Tests skip automatically when ARGUS_INTEGRATION != "1", so they do not
// break unit-only CI runs.
package behaviour_integration

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// skipIfNoIntegration skips the test unless ARGUS_INTEGRATION=1 is set.
func skipIfNoIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("ARGUS_INTEGRATION") != "1" {
		t.Skip("ARGUS_INTEGRATION!=1; skipping integration test")
	}
}

// newCH opens a ClickHouse connection from ARGUS_TEST_CH_DSN.
// Skips the test when the env var is not set.
func newCH(t *testing.T) driver.Conn {
	t.Helper()
	dsn := os.Getenv("ARGUS_TEST_CH_DSN")
	if dsn == "" {
		t.Skip("ARGUS_TEST_CH_DSN unset")
	}
	conn, err := clickhouse.Open(&clickhouse.Options{Addr: []string{dsn}})
	if err != nil {
		t.Fatalf("ch open: %v", err)
	}
	return conn
}

// newPG opens a PostgreSQL pool from ARGUS_TEST_PG_DSN.
// Skips the test when the env var is not set.
func newPG(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("ARGUS_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("ARGUS_TEST_PG_DSN unset")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pg open: %v", err)
	}
	return pool
}

// SeedSignal is a minimal fixture for inserting into the signals table.
// Only the columns used by Phase 7 queries are required; everything else
// is given safe defaults in seedSignals.
type SeedSignal struct {
	SignalID      string
	TraceID       string
	SpanID        string
	ParentSpanID  string // empty string → NULL
	AppID         string
	Category      string
	Layer         uint8
	Severity      uint8
	Timestamp     time.Time
	DurationMS    float32
	BaselineDevia float32
	SessionID     string // empty string → NULL
	ConvID        string // empty string → NULL
}

// seedSignals INSERTs a batch of signals into ClickHouse using the exact column
// names from internal/storage/schema.go (SignalsTableDDL).
func seedSignals(t *testing.T, ctx context.Context, ch driver.Conn, sigs []SeedSignal) {
	t.Helper()
	if len(sigs) == 0 {
		return
	}
	batch, err := ch.PrepareBatch(ctx, `INSERT INTO signals (
		signal_id, trace_id, span_id, parent_span_id,
		app_id, app_version,
		layer, category, severity,
		timestamp, duration_ms, ingested_at,
		session_id, conversation_id,
		enrich_baseline_deviation,
		data_classification, retention_policy, pii_detected
	) VALUES`)
	if err != nil {
		t.Fatalf("prepare batch: %v", err)
	}
	for _, s := range sigs {
		dur := s.DurationMS
		dev := s.BaselineDevia
		if err := batch.Append(
			s.SignalID, s.TraceID, s.SpanID, nullable(s.ParentSpanID),
			s.AppID, "test-1.0",
			s.Layer, s.Category, s.Severity,
			s.Timestamp, &dur, time.Now(),
			nullable(s.SessionID), nullable(s.ConvID),
			&dev,
			uint8(1), "standard", uint8(0),
		); err != nil {
			t.Fatalf("append signal %s: %v", s.SignalID, err)
		}
	}
	if err := batch.Send(); err != nil {
		t.Fatalf("send batch: %v", err)
	}
}

// nullable converts an empty string to nil, otherwise returns a pointer to the value.
func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// seedAlert inserts an alert row into the PostgreSQL alerts table using the
// schema from migrations/018_create_alerts_v2.up.sql.
// Returns the generated alert UUID.
func seedAlert(t *testing.T, ctx context.Context, pg *pgxpool.Pool, signalIDs []string) string {
	t.Helper()
	id := uuid.NewString()
	fingerprint := uuid.NewString() // unique per insertion
	_, err := pg.Exec(ctx, `
		INSERT INTO alerts
			(id, rule_id, signal_ids, fingerprint, severity, title, description, status, created_at)
		VALUES
			($1, 'test-rule', $2, $3, 3, 'Test Alert', 'Integration test alert', 'open', now())`,
		id, signalIDs, fingerprint)
	if err != nil {
		t.Fatalf("seed alert: %v", err)
	}
	return id
}

// seedSessionProfile upserts a session_baseline_profiles row in PostgreSQL.
// Uses the schema from migrations/011_session_baseline.up.sql.
func seedSessionProfile(t *testing.T, ctx context.Context, pg *pgxpool.Pool, appID string, layers []int32) {
	t.Helper()
	_, err := pg.Exec(ctx, `
		INSERT INTO session_baseline_profiles
			(app_id, layer_sequence, session_dur_p50, session_dur_p95,
			 layer_dwell_ms, anomaly_rate, sample_count, computed_at, expires_at)
		VALUES
			($1, $2, 100, 500, '{}'::jsonb, 0.02, 10, now(), now() + interval '30 minutes')
		ON CONFLICT (app_id) DO UPDATE
			SET layer_sequence = EXCLUDED.layer_sequence,
			    computed_at    = now(),
			    expires_at     = now() + interval '30 minutes'`,
		appID, layers)
	if err != nil {
		t.Fatalf("seed session profile for app %s: %v", appID, err)
	}
}

// apiURL returns the base URL of the running Argus API server.
// Defaults to http://localhost:8080 when ARGUS_TEST_API_URL is unset.
func apiURL() string {
	if u := os.Getenv("ARGUS_TEST_API_URL"); u != "" {
		return u
	}
	return "http://localhost:8080"
}

// doGET issues a GET request to path with an optional Bearer token read from
// the given environment variable name.
//   - tokenEnv = ""  → request is sent without an Authorization header (tests 401)
//   - tokenEnv set but env var unset → test is skipped
func doGET(t *testing.T, path, tokenEnv string) *http.Response {
	t.Helper()
	var token string
	if tokenEnv != "" {
		token = os.Getenv(tokenEnv)
		if token == "" {
			t.Skipf("%s unset; skipping", tokenEnv)
		}
	}
	req, err := http.NewRequest(http.MethodGet, apiURL()+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

// phase7Paths returns all Phase 7 API paths for auth-sweep tests.
// Uses placeholder IDs that will produce 404/401/403 but not 500.
var phase7Paths = []string{
	"/api/v1/traces/no-such-trace/graph",
	"/api/v1/sessions/no-such-session/timeline",
	"/api/v1/conversations/no-such-conv/behaviour",
	"/api/v1/alerts/00000000-0000-0000-0000-000000000000/chain",
	"/api/v1/traces/recent?app_id=no-such-app",
}

// suppress unused-import warning when only helpers are compiled
var _ = fmt.Sprintf
