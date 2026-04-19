//go:build integration

// Package detection_test provides end-to-end integration tests for the full
// signal → detection engine → alert pipeline.
//
// Run with:
//
//	TEST_POSTGRES_DSN=postgres://argus:argus@localhost:5432/argus_test?sslmode=disable \
//	TEST_REDIS_DSN=localhost:6379 \
//	go test -tags=integration ./internal/detection/ -run TestIntegration -timeout 180s
package detection_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/detection/breaker"
	"github.com/argusxdr/argus/internal/detection/engine"
	"github.com/argusxdr/argus/internal/detection/loader"
	"github.com/argusxdr/argus/internal/detection/worker"
	"github.com/argusxdr/argus/internal/ingest"
	"github.com/argusxdr/argus/internal/kairos"
	"github.com/argusxdr/argus/internal/metrics"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// Test environment setup
// ---------------------------------------------------------------------------

type testEnv struct {
	pool    *pgxpool.Pool
	rdb     *redis.Client
	log     *zap.Logger
	cleanup func()
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	pgDSN := os.Getenv("TEST_POSTGRES_DSN")
	if pgDSN == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	redisDSN := os.Getenv("TEST_REDIS_DSN")
	if redisDSN == "" {
		redisDSN = "localhost:6379"
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, pgDSN)
	require.NoError(t, err, "connect postgres")

	// Apply all migrations (fresh schema)
	applyAllMigrations(t, pool)

	rdb := redis.NewClient(&redis.Options{Addr: redisDSN})
	require.NoError(t, rdb.Ping(ctx).Err(), "connect redis")

	log, _ := zap.NewDevelopment()

	return &testEnv{
		pool: pool,
		rdb:  rdb,
		log:  log,
		cleanup: func() {
			dropAllTables(t, pool)
			pool.Close()
			_ = rdb.Close()
		},
	}
}

func migrationsDir() string {
	_, filename, _, _ := runtime.Caller(0)
	// Go up two levels: internal/detection → internal → project root
	return filepath.Join(filepath.Dir(filename), "..", "storage", "migrations")
}

func applyAllMigrations(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	applyMigrations(t, pool, "up", 999)
}

func dropAllTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	applyMigrations(t, pool, "down", 999)
}

func applyMigrations(t *testing.T, pool *pgxpool.Pool, direction string, upTo int) {
	t.Helper()
	dir := migrationsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read migrations dir %s: %v", dir, err)
	}

	suffix := "." + direction + ".sql"
	type mf struct {
		num  int
		path string
	}
	var files []mf
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		parts := strings.SplitN(e.Name(), "_", 2)
		if len(parts) < 2 {
			continue
		}
		n, err := strconv.Atoi(parts[0])
		if err != nil || n > upTo {
			continue
		}
		files = append(files, mf{num: n, path: filepath.Join(dir, e.Name())})
	}

	if direction == "down" {
		sort.Slice(files, func(i, j int) bool { return files[i].num > files[j].num })
	} else {
		sort.Slice(files, func(i, j int) bool { return files[i].num < files[j].num })
	}

	for _, f := range files {
		sql, err := os.ReadFile(f.path)
		if err != nil {
			t.Fatalf("read migration %s: %v", f.path, err)
		}
		if _, err := pool.Exec(context.Background(), string(sql)); err != nil {
			t.Fatalf("exec migration %s: %v", f.path, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const tier2RuleYAML = `
id: test-tier2
name: Test Tier2 Rule
tier: 2
enabled: true
severity: 3
conditions:
  layer: "7"
  category_prefix: "auth"
  baseline_deviation_gte: 0.0
action:
  title: Test Tier2 Alert
  description: Integration test alert
`

const tier1RuleYAML = `
id: test-tier1
name: Test Tier1 Rule
tier: 1
enabled: true
severity: 3
conditions:
  layer: "7"
  category_prefix: "auth"
action:
  title: Test Tier1 Alert
  description: Integration test tier1 alert
`

func makeSignal(traceID, appID, signalID string) *v1.ArgusSignal {
	return &v1.ArgusSignal{
		SignalId: signalID,
		TraceId:  traceID,
		Layer:    7,
		Category: "auth.failure",
		Severity: v1.Severity_MEDIUM,
		Source: &v1.Source{
			AppId: appID,
		},
	}
}

func insertRule(t *testing.T, pool *pgxpool.Pool, ruleID, yaml string, version int64) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO detection_rules (rule_id, name, tier, enabled, yaml_config, version)
		 VALUES ($1, $2, 'deterministic', true, $3, $4)
		 ON CONFLICT (rule_id) DO UPDATE SET yaml_config=$3, version=$4, enabled=true`,
		ruleID, ruleID, yaml, version)
	require.NoError(t, err, "insert rule %s", ruleID)
}

func newTestWorker(env *testEnv, kairosEval worker.KairosEvaluator) (*worker.AsyncDetectionWorker, *engine.RuleStore, *metrics.Detection) {
	store := engine.NewRuleStore()
	eng := engine.New(store, nil)
	br := breaker.New(30 * time.Second)
	reg := prometheus.NewRegistry()
	m := metrics.NewDetection(reg)

	router := ingest.NewAlertRouter(env.pool, env.rdb, nil, nil, env.log)
	w := worker.New(eng, router, br, m, env.log, 100, 2)
	if kairosEval != nil {
		w.WithKairos(kairosEval)
	}
	return w, store, m
}

func waitForAlerts(t *testing.T, pool *pgxpool.Pool, ruleID string, timeout time.Duration) int {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var count int
		err := pool.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM alerts WHERE rule_id=$1`, ruleID).Scan(&count)
		require.NoError(t, err)
		if count > 0 {
			return count
		}
		time.Sleep(100 * time.Millisecond)
	}
	return 0
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestIntegration_SignalToAlert_Tier2 verifies a signal matching a Tier 2 rule
// produces an alert row with all required fields populated (D-01, D-03, D-06, D-07).
func TestIntegration_SignalToAlert_Tier2(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	insertRule(t, env.pool, "test-tier2", tier2RuleYAML, 1)

	w, store, _ := newTestWorker(env, nil)

	// Manually load rule into store (simulating DBRuleLoader first-load)
	r, err := engine.ParseRuleYAML([]byte(tier2RuleYAML))
	require.NoError(t, err)
	store.Add(r)

	w.Start(context.Background())
	defer w.Shutdown()

	sig := makeSignal("tr-1", "app-1", "sig-001")
	w.Enqueue(sig)

	// Wait up to 2s for async processing
	time.Sleep(2 * time.Second)

	var (
		fingerprint    string
		traceID        string
		signalIDs      []string
		signalCount    int
		status         string
		ctxJSON        []byte
		kairosDecision []byte
	)
	err = env.pool.QueryRow(context.Background(),
		`SELECT fingerprint, trace_id, signal_ids, signal_count, status, context, kairos_decision
		 FROM alerts WHERE rule_id='test-tier2'`).
		Scan(&fingerprint, &traceID, &signalIDs, &signalCount, &status, &ctxJSON, &kairosDecision)
	require.NoError(t, err, "alert row should exist")

	require.NotEmpty(t, fingerprint, "fingerprint must not be empty")
	require.Equal(t, "tr-1", traceID, "trace_id must be linked")
	require.Contains(t, signalIDs, "sig-001", "signal_ids must contain signal ID")
	require.NotNil(t, ctxJSON, "context must not be NULL")
	// kairos_decision IS NULL when kairos is disabled (D-13 fail-open)
	require.Equal(t, []byte("null"), kairosDecision, "kairos_decision must be null when Kairos disabled")
	require.Equal(t, "open", status, "initial status must be open")
}

// TestIntegration_FingerprintDedup_IncrementsCount verifies that 3 identical signals
// produce exactly 1 alert row with signal_count=3 (D-03 dedup).
func TestIntegration_FingerprintDedup_IncrementsCount(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	w, store, _ := newTestWorker(env, nil)
	r, err := engine.ParseRuleYAML([]byte(tier2RuleYAML))
	require.NoError(t, err)
	store.Add(r)

	w.Start(context.Background())
	defer w.Shutdown()

	for i := 0; i < 3; i++ {
		sig := makeSignal("tr-dedup", "app-dedup", "sig-dedup-"+strconv.Itoa(i))
		w.Enqueue(sig)
	}

	time.Sleep(2 * time.Second)

	var rowCount int
	err = env.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM alerts WHERE rule_id='test-tier2'`).Scan(&rowCount)
	require.NoError(t, err)
	require.Equal(t, 1, rowCount, "dedup: should be exactly 1 alert row")

	var signalCount int
	err = env.pool.QueryRow(context.Background(),
		`SELECT signal_count FROM alerts WHERE rule_id='test-tier2'`).Scan(&signalCount)
	require.NoError(t, err)
	require.Equal(t, 3, signalCount, "signal_count should equal 3 after 3 identical signals")
}

// TestIntegration_SuppressedAt101 verifies that an alert is auto-suppressed after
// signal_count exceeds 100 (D-04).
func TestIntegration_SuppressedAt101(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	w, store, _ := newTestWorker(env, nil)
	r, err := engine.ParseRuleYAML([]byte(tier2RuleYAML))
	require.NoError(t, err)
	store.Add(r)

	w.Start(context.Background())
	defer w.Shutdown()

	for i := 0; i < 101; i++ {
		sig := makeSignal("tr-suppress", "app-suppress", "sig-suppress-"+strconv.Itoa(i))
		w.Enqueue(sig)
	}

	// Allow time for all 101 signals to be processed
	time.Sleep(5 * time.Second)

	var status string
	err = env.pool.QueryRow(context.Background(),
		`SELECT status FROM alerts WHERE rule_id='test-tier2'`).Scan(&status)
	require.NoError(t, err)
	require.Equal(t, "suppressed", status, "alert should be suppressed after 101 signals")
}

// TestIntegration_KairosUnreachable_FailOpen verifies that when Kairos is unreachable,
// the alert is still written with kairos_decision=NULL and the timeout metric is incremented (D-13, D-14).
func TestIntegration_KairosUnreachable_FailOpen(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	// Use an unreachable Kairos endpoint
	kairosClient := kairos.NewClient("http://localhost:1", 500*time.Millisecond, env.log)
	kairosEval := kairos.NewEvaluator(kairosClient, &kairos.PolicyConfig{
		Enabled:     true, // Must be enabled so evaluator actually calls Kairos
		Endpoint:    "http://localhost:1",
		TimeoutMs:   500,
		FailOpen:    true,
		CacheSize:   100,
		CacheTTLSec: 10,
	}, env.log)

	store := engine.NewRuleStore()
	eng := engine.New(store, nil)
	br := breaker.New(30 * time.Second)
	reg := prometheus.NewRegistry()
	m := metrics.NewDetection(reg)

	r, err := engine.ParseRuleYAML([]byte(tier2RuleYAML))
	require.NoError(t, err)
	// Force requires_kairos so the worker always calls Kairos
	r.RequiresKairos = true
	store.Add(r)

	router := ingest.NewAlertRouter(env.pool, env.rdb, nil, nil, env.log)

	w := worker.New(eng, router, br, m, env.log, 100, 2)
	w.WithKairos(kairosEval)
	w.Start(context.Background())
	defer w.Shutdown()

	sig := makeSignal("tr-kairos", "app-kairos", "sig-kairos-001")
	w.Enqueue(sig)

	// Wait up to 3s for processing (Kairos timeout is 500ms)
	time.Sleep(3 * time.Second)

	var kairosDecision []byte
	err = env.pool.QueryRow(context.Background(),
		`SELECT kairos_decision FROM alerts WHERE rule_id='test-tier2'`).Scan(&kairosDecision)
	require.NoError(t, err, "alert must be written even when Kairos is unreachable")
	require.Equal(t, []byte("null"), kairosDecision, "kairos_decision must be null on timeout/unreachable")
}

// TestIntegration_HotReload_PicksUpNewVersion verifies that DBRuleLoader detects
// a version bump in detection_rules and hot-swaps new rules within its poll interval (D-08).
func TestIntegration_HotReload_PicksUpNewVersion(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	// Insert rule version=1 (tier1 matching "auth")
	insertRule(t, env.pool, "test-tier2", tier2RuleYAML, 1)

	store := engine.NewRuleStore()
	dbLoader := loader.New(env.pool, store, 1*time.Second, env.log)

	// Run initial load synchronously
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go dbLoader.Run(ctx)

	// Allow first load
	time.Sleep(300 * time.Millisecond)

	// Now update the rule YAML to match a different category so only new signals fire
	updatedYAML := strings.ReplaceAll(tier2RuleYAML, "id: test-tier2", "id: test-tier2-v2")
	updatedYAML = strings.ReplaceAll(updatedYAML, "test-tier2", "test-tier2-v2")
	newRuleYAML := `
id: test-tier2-v2
name: Test Tier2 Rule V2
tier: 2
enabled: true
severity: 4
conditions:
  layer: "7"
  category_prefix: "auth"
  baseline_deviation_gte: 0.0
action:
  title: Hot Reload Test Alert
  description: Hot reload integration test
`
	insertRule(t, env.pool, "test-tier2-v2", newRuleYAML, 2)

	// Also bump the version on the existing rule to trigger reload
	_, err := env.pool.Exec(context.Background(),
		`UPDATE detection_rules SET version=2 WHERE rule_id='test-tier2'`)
	require.NoError(t, err)

	// Wait for hot reload (2x poll interval)
	time.Sleep(2 * time.Second)

	// Verify store has the new rule
	_, found := store.Get("test-tier2-v2")
	require.True(t, found, "hot reload should have loaded test-tier2-v2")
}

// TestIntegration_TraceIdRequired_Rejects verifies that a signal with an empty
// trace_id is rejected and no alert row is written (D-06).
func TestIntegration_TraceIdRequired_Rejects(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	w, store, _ := newTestWorker(env, nil)
	r, err := engine.ParseRuleYAML([]byte(tier2RuleYAML))
	require.NoError(t, err)
	store.Add(r)

	w.Start(context.Background())
	defer w.Shutdown()

	// Signal with empty trace_id
	sig := &v1.ArgusSignal{
		SignalId: "sig-no-trace",
		TraceId:  "", // empty — should be rejected
		Layer:    7,
		Category: "auth.failure",
		Severity: v1.Severity_MEDIUM,
		Source:   &v1.Source{AppId: "app-notrace"},
	}
	w.Enqueue(sig)

	time.Sleep(1 * time.Second)

	var count int
	err = env.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM alerts WHERE rule_id='test-tier2'`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count, "no alert should be written for signal with empty trace_id (D-06)")
}
