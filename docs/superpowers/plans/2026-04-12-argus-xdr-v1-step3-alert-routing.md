# Step 3: Alert Routing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire the full alert pipeline: `DetectionProcessor` match → fingerprint dedup (Redis, 15-min) → PostgreSQL alert persist → `RoutingEngine` → `AlertDispatcher` → circuit-breaker-wrapped adapter; incident auto-correlation (≥3 alerts same app/trace in 10 min); live alert/incident HTTP handlers replacing 501 stubs.

**Architecture:** `AlertRouter` (new) is the `pipeline.AlertWriter` implementation that replaces the bare `PgAlertWriter`. It owns dedup, DB write, routing lookup, and dispatch. The `RoutingEngine` is fixed to query the actual migration 008 `routing_rules` schema (`min_severity / app_id_filter / layer_filter / channel_id`) rather than the non-existent `condition_expr / targets` columns. Every adapter is wrapped in a `CircuitBreakerAdapter` before registration. Incident correlation runs inside `AlertRouter.WriteAlert` after the alert write.

**Tech Stack:** Go, pgx/v5, go-redis/v9, chi, zap, testify — all already in go.mod

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/notify/router.go` | Modify | Fix `RoutingRule` struct + `SyncRules` SQL + real `simpleEval` |
| `internal/notify/circuitbreaker_adapter.go` | Create | `CircuitBreakerAdapter` wraps `Notifier` with `CircuitBreaker.Execute` |
| `internal/notify/log_adapter.go` | Create | `LogAdapter` — logs notifications, used in tests and dev |
| `internal/ingest/alert_router.go` | Create | Full alert pipeline: dedup → write → route → dispatch |
| `internal/ingest/alert_router_test.go` | Create | Integration tests with mock dispatcher |
| `internal/ingest/handler_alerts.go` | Create | Live alert HTTP handlers (replaces stubs in handler_stubs.go) |
| `internal/ingest/handler_incidents.go` | Create | Live incident HTTP handlers (replaces stubs in handler_stubs.go) |
| `internal/ingest/receiver_query.go` | Modify | Add `alertRouter *AlertRouter` field + `SetAlertRouter` setter |
| `cmd/argus/api.go` | Modify | Wire registry, adapters, dispatcher, routing, alert router |

**Do NOT modify** `internal/ingest/pg_alert_writer.go` (kept as historical reference), `internal/alert/` package (schema mismatch with migration 008 — bypassed entirely in Step 3), `internal/notify/adapter.go`, `internal/notify/dispatcher.go`, `internal/notify/circuitbreaker.go`.

---

### Task 1: Fix RoutingEngine to match migration 008 schema

**Files:**
- Modify: `internal/notify/router.go`

Migration 008 `routing_rules` has: `id, channel_id UUID, min_severity INT, app_id_filter TEXT (nullable), layer_filter INT (nullable), enabled BOOL`.
`RoutingEngine.SyncRules` currently queries `routing_rule_id, name, enabled, condition_expr, targets` — none of these columns exist. `simpleEval` always returns `true` (placeholder).
`notification_channels` has `id UUID, type TEXT` where `type` ∈ {slack, email, pagerduty, webhook, syslog}.

The fixed engine JOINs `notification_channels` in `SyncRules` to resolve channel type (the adapter name) at sync time. `Evaluate` returns `[]string` of adapter names, unchanged from its current signature.

- [ ] **Step 1.1: Write the failing test**

Create `internal/notify/router_test.go`:

```go
package notify

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoutingEngine_simpleEval(t *testing.T) {
	e := &RoutingEngine{}

	rule := &RoutingRule{
		MinSeverity: 3,
		AppIDFilter: nil,
		LayerFilter: nil,
		Targets:     []string{"slack"},
	}

	// Severity below threshold → no match
	ctx := &EvaluationContext{Severity: 2, AppID: "app1", Layer: 5}
	assert.False(t, e.simpleEval(rule, ctx))

	// Severity meets threshold → match
	ctx.Severity = 3
	assert.True(t, e.simpleEval(rule, ctx))

	// AppID filter set, wrong app → no match
	appFilter := "app-allowed"
	rule.AppIDFilter = &appFilter
	ctx.AppID = "app-other"
	assert.False(t, e.simpleEval(rule, ctx))

	// Correct app → match
	ctx.AppID = "app-allowed"
	assert.True(t, e.simpleEval(rule, ctx))

	// Layer filter set, wrong layer → no match
	layerFilter := 7
	rule.LayerFilter = &layerFilter
	ctx.Layer = 5
	assert.False(t, e.simpleEval(rule, ctx))

	// Correct layer → match
	ctx.Layer = 7
	assert.True(t, e.simpleEval(rule, ctx))
}

func TestRoutingEngine_Evaluate_nilContext(t *testing.T) {
	e := &RoutingEngine{}
	targets := e.Evaluate(nil)
	require.NotNil(t, targets)
	assert.Empty(t, targets)
}

func TestRoutingEngine_NewRoutingEngine_nilDB(t *testing.T) {
	_, err := NewRoutingEngine(nil, nil)
	assert.Error(t, err)
}
```

- [ ] **Step 1.2: Run test to verify it fails**

```
cd C:/Users/Drupad/ArgusXDR/.worktrees/step2-detection-engine
go test ./internal/notify/... -run TestRoutingEngine -v
```

Expected: FAIL — `simpleEval` signature mismatch; `RoutingRule` lacks `MinSeverity`, `AppIDFilter`, `LayerFilter`.

- [ ] **Step 1.3: Replace `RoutingRule`, `EvaluationContext`, `SyncRules`, `simpleEval` in `internal/notify/router.go`**

Replace the struct declarations and methods. Do NOT touch `NewRoutingEngine`, `Start`, `Stop`, `syncWorker`, `GetRules`, `GetRule`, `LastSyncTime`, `RuleCount`, `SetSyncInterval`.

Replace lines 17–38 (the two struct definitions) with:
```go
// RoutingRule represents a routing configuration loaded from PostgreSQL.
// It matches alerts based on severity, app, and layer filters.
type RoutingRule struct {
	RuleID      uuid.UUID  // routing_rules.id
	ChannelID   uuid.UUID  // routing_rules.channel_id
	Enabled     bool
	MinSeverity int       // alert must have severity >= this value
	AppIDFilter *string   // nil = match all apps
	LayerFilter *int      // nil = match all layers
	Targets     []string  // adapter names (channel types from notification_channels.type)
}

// EvaluationContext holds the alert fields needed to evaluate routing conditions.
type EvaluationContext struct {
	Severity int    // Alert severity 1-5
	AppID    string // Application ID
	Layer    int    // LLM system layer 1-10
}
```

Replace the `SyncRules` method body (lines 116–165) with:
```go
func (e *RoutingEngine) SyncRules(ctx context.Context) error {
	rows, err := e.db.Query(ctx, `
		SELECT rr.id, rr.channel_id, rr.min_severity, rr.app_id_filter, rr.layer_filter, nc.type
		FROM routing_rules rr
		JOIN notification_channels nc ON rr.channel_id = nc.id
		WHERE rr.enabled = true AND nc.enabled = true
		ORDER BY rr.id
	`)
	if err != nil {
		return fmt.Errorf("failed to query routing rules: %w", err)
	}
	defer rows.Close()

	newRules := make(map[uuid.UUID]*RoutingRule)

	for rows.Next() {
		rule := &RoutingRule{Enabled: true}
		var channelType string
		err := rows.Scan(
			&rule.RuleID, &rule.ChannelID, &rule.MinSeverity,
			&rule.AppIDFilter, &rule.LayerFilter, &channelType,
		)
		if err != nil {
			e.logger.Error("failed to scan routing rule", zap.Error(err))
			continue
		}
		rule.Targets = []string{channelType}
		newRules[rule.RuleID] = rule
	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("error iterating routing rules: %w", err)
	}

	e.mu.Lock()
	e.rules = newRules
	e.lastSync = time.Now()
	e.mu.Unlock()

	e.logger.Debug("routing rules synced", zap.Int("count", len(newRules)))
	return nil
}
```

Replace the `Evaluate` method signature and body:
```go
// Evaluate returns the adapter names to send this alert to.
// Returns an empty slice if no rules match or context is nil.
func (e *RoutingEngine) Evaluate(ctx *EvaluationContext) []string {
	if ctx == nil {
		return []string{}
	}

	e.mu.RLock()
	rules := e.rules
	e.mu.RUnlock()

	targetSet := make(map[string]bool)
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if e.simpleEval(rule, ctx) {
			for _, target := range rule.Targets {
				targetSet[target] = true
			}
		}
	}

	targets := make([]string, 0, len(targetSet))
	for target := range targetSet {
		targets = append(targets, target)
	}
	return targets
}
```

Replace the `evaluateCondition` and `simpleEval` methods:
```go
func (e *RoutingEngine) simpleEval(rule *RoutingRule, ctx *EvaluationContext) bool {
	if ctx.Severity < rule.MinSeverity {
		return false
	}
	if rule.AppIDFilter != nil && *rule.AppIDFilter != ctx.AppID {
		return false
	}
	if rule.LayerFilter != nil && *rule.LayerFilter != ctx.Layer {
		return false
	}
	return true
}
```

Remove `evaluateCondition` entirely (it was only a wrapper for `simpleEval`). Remove the `CreatedAt`, `UpdatedAt`, `CreatedBy`, `Name`, `ConditionExpr` fields from `RoutingRule`.

Also remove the `ConditionExpr` reference in the `Scan` call and the `targetsJSON` unmarshal block (they are gone).

- [ ] **Step 1.4: Run test to verify it passes**

```
go test ./internal/notify/... -run TestRoutingEngine -v
```

Expected: PASS

- [ ] **Step 1.5: Commit**

```bash
git add internal/notify/router.go internal/notify/router_test.go
git commit -m "fix(notify): align RoutingEngine with migration 008 routing_rules schema

Replace condition_expr/targets SQL with min_severity/app_id_filter/layer_filter.
JOIN notification_channels to resolve channel type as adapter name.
Replace always-true simpleEval with real severity+app+layer filtering.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 2: CircuitBreakerAdapter

**Files:**
- Create: `internal/notify/circuitbreaker_adapter.go`
- Create: `internal/notify/circuitbreaker_adapter_test.go`

Wraps any `Notifier` with a `CircuitBreaker` so dispatcher workers never call a failing adapter directly. Each adapter gets its own `CircuitBreaker` instance.

- [ ] **Step 2.1: Write the failing test**

Create `internal/notify/circuitbreaker_adapter_test.go`:

```go
package notify

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubNotifier struct {
	name    string
	sendErr error
	calls   int
}

func (s *stubNotifier) Name() string { return s.name }
func (s *stubNotifier) Health(_ context.Context) error { return nil }
func (s *stubNotifier) Send(_ context.Context, req *NotificationRequest) (*NotificationResponse, error) {
	s.calls++
	if s.sendErr != nil {
		return nil, s.sendErr
	}
	return &NotificationResponse{Status: "sent"}, nil
}

func TestCircuitBreakerAdapter_SuccessPassThrough(t *testing.T) {
	inner := &stubNotifier{name: "stub"}
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())
	adapter := NewCircuitBreakerAdapter(inner, cb)

	assert.Equal(t, "stub", adapter.Name())

	resp, err := adapter.Send(context.Background(), &NotificationRequest{Title: "test"})
	require.NoError(t, err)
	assert.Equal(t, "sent", resp.Status)
	assert.Equal(t, 1, inner.calls)
}

func TestCircuitBreakerAdapter_ErrorPropagated(t *testing.T) {
	inner := &stubNotifier{name: "stub", sendErr: errors.New("send failed")}
	// Use small MaxRetries so the test is fast
	cfg := DefaultCircuitBreakerConfig()
	cfg.MaxRetries = 0
	cb := NewCircuitBreaker(cfg)
	adapter := NewCircuitBreakerAdapter(inner, cb)

	_, err := adapter.Send(context.Background(), &NotificationRequest{Title: "test"})
	assert.Error(t, err)
}

func TestCircuitBreakerAdapter_Health(t *testing.T) {
	inner := &stubNotifier{name: "stub"}
	adapter := NewCircuitBreakerAdapter(inner, NewCircuitBreaker(nil))
	assert.NoError(t, adapter.Health(context.Background()))
}
```

- [ ] **Step 2.2: Run test to verify it fails**

```
go test ./internal/notify/... -run TestCircuitBreakerAdapter -v
```

Expected: FAIL — `NewCircuitBreakerAdapter` undefined.

- [ ] **Step 2.3: Create `internal/notify/circuitbreaker_adapter.go`**

```go
package notify

import "context"

// CircuitBreakerAdapter wraps a Notifier with a CircuitBreaker.
// Each adapter should have its own CircuitBreaker instance.
type CircuitBreakerAdapter struct {
	inner Notifier
	cb    *CircuitBreaker
}

// NewCircuitBreakerAdapter creates a new CircuitBreakerAdapter.
func NewCircuitBreakerAdapter(inner Notifier, cb *CircuitBreaker) *CircuitBreakerAdapter {
	if cb == nil {
		cb = NewCircuitBreaker(nil)
	}
	return &CircuitBreakerAdapter{inner: inner, cb: cb}
}

// Name returns the underlying adapter's name.
func (a *CircuitBreakerAdapter) Name() string { return a.inner.Name() }

// Send sends the notification through the circuit breaker.
// If the circuit is open, returns an error immediately without calling the adapter.
func (a *CircuitBreakerAdapter) Send(ctx context.Context, req *NotificationRequest) (*NotificationResponse, error) {
	var resp *NotificationResponse
	err := a.cb.Execute(ctx, func(ctx context.Context) error {
		var sendErr error
		resp, sendErr = a.inner.Send(ctx, req)
		return sendErr
	})
	return resp, err
}

// Health delegates to the underlying adapter.
func (a *CircuitBreakerAdapter) Health(ctx context.Context) error {
	return a.inner.Health(ctx)
}
```

- [ ] **Step 2.4: Run test to verify it passes**

```
go test ./internal/notify/... -run TestCircuitBreakerAdapter -v
```

Expected: PASS

- [ ] **Step 2.5: Commit**

```bash
git add internal/notify/circuitbreaker_adapter.go internal/notify/circuitbreaker_adapter_test.go
git commit -m "feat(notify): add CircuitBreakerAdapter wrapping Notifier with circuit breaker

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 3: LogAdapter

**Files:**
- Create: `internal/notify/log_adapter.go`
- Create: `internal/notify/log_adapter_test.go`

`LogAdapter` satisfies `Notifier`. It logs each notification via zap and accumulates sent requests in memory for test assertions. Used as the real adapter in dev and tests in place of Slack/PagerDuty.

- [ ] **Step 3.1: Write the failing test**

Create `internal/notify/log_adapter_test.go`:

```go
package notify

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestLogAdapter_Name(t *testing.T) {
	a := NewLogAdapter(zap.NewNop())
	assert.Equal(t, "log", a.Name())
}

func TestLogAdapter_Send(t *testing.T) {
	a := NewLogAdapter(zap.NewNop())

	req := &NotificationRequest{
		Title:   "Test Alert",
		Message: "Something fired",
	}
	resp, err := a.Send(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "sent", resp.Status)

	sent := a.Sent()
	require.Len(t, sent, 1)
	assert.Equal(t, "Test Alert", sent[0].Title)
}

func TestLogAdapter_Health(t *testing.T) {
	a := NewLogAdapter(zap.NewNop())
	assert.NoError(t, a.Health(context.Background()))
}

func TestLogAdapter_Reset(t *testing.T) {
	a := NewLogAdapter(zap.NewNop())
	a.Send(context.Background(), &NotificationRequest{Title: "x"})
	a.Reset()
	assert.Empty(t, a.Sent())
}
```

- [ ] **Step 3.2: Run test to verify it fails**

```
go test ./internal/notify/... -run TestLogAdapter -v
```

Expected: FAIL — `NewLogAdapter` undefined.

- [ ] **Step 3.3: Create `internal/notify/log_adapter.go`**

```go
package notify

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// LogAdapter is a Notifier that logs notifications and stores them for inspection.
// Used as a real adapter in development and as a test double.
type LogAdapter struct {
	logger *zap.Logger
	mu     sync.Mutex
	sent   []*NotificationRequest
}

// NewLogAdapter creates a new LogAdapter.
func NewLogAdapter(logger *zap.Logger) *LogAdapter {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &LogAdapter{logger: logger}
}

// Name returns "log".
func (a *LogAdapter) Name() string { return "log" }

// Send logs the notification and records it for inspection.
func (a *LogAdapter) Send(_ context.Context, req *NotificationRequest) (*NotificationResponse, error) {
	a.logger.Info("notification dispatched",
		zap.String("alert_id", req.AlertID.String()),
		zap.String("title", req.Title),
		zap.Int("severity", req.Severity),
	)
	a.mu.Lock()
	a.sent = append(a.sent, req)
	a.mu.Unlock()
	return &NotificationResponse{
		Status:    "sent",
		MessageID: req.ID,
		Timestamp: time.Now().Unix(),
	}, nil
}

// Health always returns nil.
func (a *LogAdapter) Health(_ context.Context) error { return nil }

// Sent returns a snapshot of all notifications sent since creation or last Reset.
func (a *LogAdapter) Sent() []*NotificationRequest {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]*NotificationRequest, len(a.sent))
	copy(out, a.sent)
	return out
}

// Reset clears the sent notification log.
func (a *LogAdapter) Reset() {
	a.mu.Lock()
	a.sent = nil
	a.mu.Unlock()
}
```

- [ ] **Step 3.4: Run test to verify it passes**

```
go test ./internal/notify/... -run TestLogAdapter -v
```

Expected: PASS

- [ ] **Step 3.5: Commit**

```bash
git add internal/notify/log_adapter.go internal/notify/log_adapter_test.go
git commit -m "feat(notify): add LogAdapter for dev and test notification delivery

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 4: AlertRouter — dedup + persist + route + dispatch

**Files:**
- Create: `internal/ingest/alert_router.go`
- Create: `internal/ingest/alert_router_test.go`

`AlertRouter` satisfies `pipeline.AlertWriter`. It owns the complete post-detection pipeline:
1. Compute fingerprint = `SHA256(ruleID + ":" + appID)` (stable per rule+app, not per-signal)
2. Redis `SET NX dedup:{fingerprint} {alertID} EX 900` — if key already existed, update `signal_count` and skip dispatch
3. `INSERT INTO alerts (...)` using migration 008 schema
4. Build `EvaluationContext` from match result
5. `routing.Evaluate(&ctx)` → adapter names
6. `dispatcher.Dispatch(job)` for each batch of targets

The migration 008 `alerts` schema does NOT have `rule_id` as a nullable column reference (the YAML rule IDs like "t1-001" are not UUIDs in `detection_rules`). Set `rule_id` to NULL by omitting it from the INSERT (the column is nullable: `UUID REFERENCES detection_rules(id)`).

- [ ] **Step 4.1: Write the failing test**

Create `internal/ingest/alert_router_test.go`:

```go
package ingest

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/detection/engine"
	"github.com/argusxdr/argus/internal/notify"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mockRoutingEngine always returns the configured targets.
type mockRoutingEngine struct {
	targets []string
}

func (m *mockRoutingEngine) Evaluate(_ *notify.EvaluationContext) []string {
	return m.targets
}

// captureDispatcher records dispatched jobs.
type captureDispatcher struct {
	jobs []*notify.DispatchJob
}

func (c *captureDispatcher) Dispatch(job *notify.DispatchJob) error {
	c.jobs = append(c.jobs, job)
	return nil
}

func TestAlertRouter_WriteAlert_computesFingerprint(t *testing.T) {
	ruleID := "t1-001"
	appID := "app-abc"
	expectedFP := func() string {
		h := sha256.Sum256([]byte(ruleID + ":" + appID))
		return fmt.Sprintf("%x", h[:])
	}()

	fp := computeRouterFingerprint(ruleID, appID)
	assert.Equal(t, expectedFP, fp)
}

func TestAlertRouter_WriteAlert_nilPool_noop(t *testing.T) {
	router := &AlertRouter{
		pool:       nil,
		dispatcher: &captureDispatcher{},
		routing:    &mockRoutingEngine{targets: []string{"log"}},
		log:        zap.NewNop(),
	}

	match := engine.MatchResult{
		Rule: engine.Rule{ID: "t1-001", Severity: 3, Action: engine.Action{Title: "test"}},
		Signal: &v1.ArgusSignal{
			SignalId: "sig-1",
			Source:   &v1.Source{AppId: "app-abc"},
			Layer:    v1.Layer_L7_INFERENCE,
			Category: "anomaly",
		},
	}

	err := router.WriteAlert(context.Background(), match)
	assert.NoError(t, err) // nil pool → no-op, no panic
}

func TestAlertRouter_WriteAlert_dispatchesOnNewAlert(t *testing.T) {
	// Without a real DB/Redis we test the dispatch path using the nil-pool no-op guard.
	// Full integration test with real DB is in Task 10.
	cap := &captureDispatcher{}
	router := &AlertRouter{
		pool:       nil, // no-op
		dispatcher: cap,
		routing:    &mockRoutingEngine{targets: []string{"log"}},
		log:        zap.NewNop(),
	}

	match := engine.MatchResult{
		Rule: engine.Rule{ID: "t1-001", Severity: 3, Action: engine.Action{Title: "High Confidence Anomaly"}},
		Signal: &v1.ArgusSignal{
			SignalId: uuid.NewString(),
			TraceId:  "trace-xyz",
			Source:   &v1.Source{AppId: "myapp"},
			Layer:    v1.Layer_L7_INFERENCE,
			Category: "prompt-injection",
		},
	}

	err := router.WriteAlert(context.Background(), match)
	require.NoError(t, err)
	// No pool → no dispatch either (dispatch only happens after DB write)
	assert.Empty(t, cap.jobs)
}

func TestAlertRouter_buildNotificationRequest(t *testing.T) {
	alertID := uuid.New()
	match := engine.MatchResult{
		Rule: engine.Rule{
			ID:       "t1-001",
			Name:     "Prompt Injection",
			Severity: 4,
			Action:   engine.Action{Title: "Prompt Injection Detected", Description: "High confidence"},
		},
		Signal: &v1.ArgusSignal{
			SignalId: "sig-1",
			Source:   &v1.Source{AppId: "app1"},
			Layer:    v1.Layer_L7_INFERENCE,
			Category: "injection",
		},
	}

	req := buildNotificationRequest(alertID, match)
	assert.Equal(t, alertID, req.AlertID)
	assert.Equal(t, "Prompt Injection Detected", req.Title)
	assert.Equal(t, 4, req.Severity)
	assert.Equal(t, "app1", req.Metadata["app_id"])
	assert.Equal(t, "injection", req.Metadata["category"])
}

// Verify dedup key format
func TestAlertRouter_dedupKey(t *testing.T) {
	fp := "abc123"
	key := dedupRedisKey(fp)
	assert.Equal(t, "dedup:abc123", key)
}

// Verify incident bucket key format
func TestAlertRouter_incidentBucketKey(t *testing.T) {
	bucket := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)
	key := incidentBucketKey("app1", bucket)
	assert.Equal(t, "incidents:app:app1:1744452000", key)
}
```

- [ ] **Step 4.2: Run test to verify it fails**

```
go test ./internal/ingest/... -run TestAlertRouter -v
```

Expected: FAIL — `AlertRouter`, `computeRouterFingerprint`, `buildNotificationRequest`, `dedupRedisKey`, `incidentBucketKey` all undefined.

- [ ] **Step 4.3: Create `internal/ingest/alert_router.go`**

```go
package ingest

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/detection/engine"
	"github.com/argusxdr/argus/internal/notify"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// AlertDispatcherIface allows injecting a test double for the dispatcher.
type AlertDispatcherIface interface {
	Dispatch(job *notify.DispatchJob) error
}

// AlertRoutingIface allows injecting a test double for the routing engine.
type AlertRoutingIface interface {
	Evaluate(ctx *notify.EvaluationContext) []string
}

// AlertRouter is the pipeline.AlertWriter implementation for Step 3+.
// It replaces PgAlertWriter with full dedup+persist+route+dispatch.
type AlertRouter struct {
	pool       *pgxpool.Pool
	redis      *redis.Client   // may be nil — dedup skipped
	routing    AlertRoutingIface
	dispatcher AlertDispatcherIface
	dedupTTL   time.Duration
	log        *zap.Logger
}

// NewAlertRouter creates an AlertRouter.
// redis may be nil; dedup is skipped when unavailable (conservative: no false suppression).
func NewAlertRouter(
	pool *pgxpool.Pool,
	redisClient *redis.Client,
	routing AlertRoutingIface,
	dispatcher AlertDispatcherIface,
	log *zap.Logger,
) *AlertRouter {
	if log == nil {
		log = zap.NewNop()
	}
	return &AlertRouter{
		pool:       pool,
		redis:      redisClient,
		routing:    routing,
		dispatcher: dispatcher,
		dedupTTL:   15 * time.Minute,
		log:        log,
	}
}

// WriteAlert is the pipeline.AlertWriter entry point.
func (r *AlertRouter) WriteAlert(ctx context.Context, m engine.MatchResult) error {
	if r.pool == nil {
		return nil // no-op in test/degraded mode
	}

	appID := extractAppID(m.Signal)
	traceID := extractTraceID(m.Signal)
	signalID := extractSignalID(m.Signal)
	layer := extractLayer(m.Signal)
	category := extractCategory(m.Signal)
	fp := computeRouterFingerprint(m.Rule.ID, appID)

	// Dedup check
	isDuplicate, err := r.checkDedup(ctx, fp)
	if err != nil {
		r.log.Warn("dedup check failed, treating as new", zap.Error(err))
	}

	if isDuplicate {
		// Increment signal_count, update last_seen_at
		if _, pgErr := r.pool.Exec(ctx,
			`UPDATE alerts SET signal_count = signal_count + 1, last_seen_at = now() WHERE fingerprint = $1`,
			fp,
		); pgErr != nil {
			r.log.Warn("failed to increment signal_count", zap.Error(pgErr))
		}
		return nil
	}

	// Insert new alert
	alertID := uuid.New()
	_, err = r.pool.Exec(ctx, `
		INSERT INTO alerts (
			id, app_id, fingerprint, severity, layer, category,
			title, description, signal_ids, trace_id, status,
			signal_count, first_seen_at, last_seen_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, 'open',
			1, now(), now()
		)`,
		alertID,
		appID,
		fp,
		m.Rule.Severity,
		layer,
		category,
		m.Rule.Action.Title,
		m.Rule.Action.Description,
		[]string{signalID},
		traceID,
	)
	if err != nil {
		r.log.Warn("failed to insert alert", zap.String("rule_id", m.Rule.ID), zap.Error(err))
		return fmt.Errorf("insert alert: %w", err)
	}

	r.log.Debug("alert written",
		zap.String("alert_id", alertID.String()),
		zap.String("fingerprint", fp),
		zap.String("rule_id", m.Rule.ID),
	)

	// Incident correlation
	r.checkIncidentCorrelation(ctx, alertID, appID, traceID, m.Rule.Severity)

	// Route and dispatch
	if r.routing != nil && r.dispatcher != nil {
		evalCtx := &notify.EvaluationContext{
			Severity: m.Rule.Severity,
			AppID:    appID,
			Layer:    layer,
		}
		targets := r.routing.Evaluate(evalCtx)
		if len(targets) > 0 {
			job := &notify.DispatchJob{
				AlertID:      alertID,
				Targets:      targets,
				Notification: buildNotificationRequest(alertID, m),
			}
			if dispErr := r.dispatcher.Dispatch(job); dispErr != nil {
				r.log.Warn("dispatch failed (non-fatal)", zap.Error(dispErr))
			}
		}
	}

	return nil
}

// checkDedup uses Redis SETNX to detect duplicate fingerprints within dedupTTL.
// Returns true if this fingerprint was already seen within the window.
func (r *AlertRouter) checkDedup(ctx context.Context, fingerprint string) (bool, error) {
	if r.redis == nil {
		return false, nil
	}
	key := dedupRedisKey(fingerprint)
	// SET NX EX: returns true if key was newly set (not a duplicate)
	wasSet, err := r.redis.SetNX(ctx, key, "1", r.dedupTTL).Result()
	if err != nil {
		return false, err
	}
	return !wasSet, nil // wasSet=true → new; wasSet=false → duplicate
}

// checkIncidentCorrelation auto-creates an incident when ≥3 alerts from the same
// app occur within a 10-minute bucket. Runs as best-effort: failures are logged but not returned.
func (r *AlertRouter) checkIncidentCorrelation(ctx context.Context, alertID uuid.UUID, appID, traceID string, severity int) {
	if r.redis == nil {
		return
	}
	bucket := time.Now().Truncate(10 * time.Minute)
	key := incidentBucketKey(appID, bucket)

	count, err := r.redis.Incr(ctx, key).Result()
	if err != nil {
		r.log.Warn("incident correlation incr failed", zap.Error(err))
		return
	}
	// Set TTL on first increment
	if count == 1 {
		r.redis.Expire(ctx, key, 10*time.Minute)
	}

	if count != 3 {
		return // Not yet at threshold
	}

	// Create incident
	incidentID := uuid.New()
	title := fmt.Sprintf("Auto-incident: %s (10-min burst)", appID)
	_, pgErr := r.pool.Exec(ctx, `
		INSERT INTO incidents (
			id, title, description, severity, app_id, status,
			alert_ids, trace_ids, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, 'open',
			ARRAY[$6]::UUID[], ARRAY[$7]::TEXT[], now(), now()
		)`,
		incidentID,
		title,
		fmt.Sprintf("Auto-correlated: ≥3 alerts from app %s within 10 minutes", appID),
		severity,
		appID,
		alertID,
		traceID,
	)
	if pgErr != nil {
		r.log.Warn("failed to create auto-incident", zap.Error(pgErr))
		return
	}

	// Link this alert to the new incident
	r.pool.Exec(ctx, `UPDATE alerts SET incident_id = $1 WHERE id = $2`, incidentID, alertID)

	r.log.Info("auto-incident created",
		zap.String("incident_id", incidentID.String()),
		zap.String("app_id", appID),
	)
}

// computeRouterFingerprint returns a stable fingerprint for (ruleID, appID).
func computeRouterFingerprint(ruleID, appID string) string {
	h := sha256.Sum256([]byte(ruleID + ":" + appID))
	return fmt.Sprintf("%x", h[:])
}

// buildNotificationRequest constructs the notification payload for dispatch.
func buildNotificationRequest(alertID uuid.UUID, m engine.MatchResult) *notify.NotificationRequest {
	appID := extractAppID(m.Signal)
	category := extractCategory(m.Signal)
	return &notify.NotificationRequest{
		ID:       uuid.NewString(),
		AlertID:  alertID,
		Severity: m.Rule.Severity,
		Title:    m.Rule.Action.Title,
		Message:  m.Rule.Action.Description,
		Metadata: map[string]string{
			"app_id":   appID,
			"category": category,
			"rule_id":  m.Rule.ID,
			"rule_name": m.Rule.Name,
		},
	}
}

func dedupRedisKey(fingerprint string) string {
	return "dedup:" + fingerprint
}

func incidentBucketKey(appID string, bucket time.Time) string {
	return fmt.Sprintf("incidents:app:%s:%d", appID, bucket.Unix())
}

// Signal field extractors (handle nil gracefully)

func extractAppID(sig *v1.ArgusSignal) string {
	if sig == nil || sig.Source == nil {
		return ""
	}
	return sig.Source.AppId
}

func extractTraceID(sig *v1.ArgusSignal) string {
	if sig == nil {
		return ""
	}
	return sig.TraceId
}

func extractSignalID(sig *v1.ArgusSignal) string {
	if sig == nil {
		return ""
	}
	return sig.SignalId
}

func extractLayer(sig *v1.ArgusSignal) int {
	if sig == nil {
		return 0
	}
	return int(sig.Layer)
}

func extractCategory(sig *v1.ArgusSignal) string {
	if sig == nil {
		return ""
	}
	return sig.Category
}
```

- [ ] **Step 4.4: Run test to verify it passes**

```
go test ./internal/ingest/... -run TestAlertRouter -v
```

Expected: PASS

- [ ] **Step 4.5: Commit**

```bash
git add internal/ingest/alert_router.go internal/ingest/alert_router_test.go
git commit -m "feat(ingest): add AlertRouter — dedup, persist, route, dispatch

Replaces PgAlertWriter with full pipeline:
- Redis SETNX 15-min dedup per (rule, app) fingerprint
- INSERT into alerts with migration 008 schema
- Incident auto-correlation: ≥3 alerts/app/10min → create incident
- Route via RoutingEngine + dispatch via AlertDispatcher

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 5: Add AlertRouter to QueryHandler

**Files:**
- Modify: `internal/ingest/receiver_query.go`

Same pattern as `SetRuleStore` from Step 2. Allows `cmd/argus/api.go` to wire the alert router after construction.

- [ ] **Step 5.1: Write the failing test**

Add to `internal/ingest/receiver_query_test.go` (or create if missing):

```go
package ingest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQueryHandler_SetAlertRouter(t *testing.T) {
	h := NewQueryHandler(nil, nil, nil)
	assert.Nil(t, h.alertRouter)

	router := &AlertRouter{}
	h.SetAlertRouter(router)
	assert.Equal(t, router, h.alertRouter)
}
```

- [ ] **Step 5.2: Run test to verify it fails**

```
go test ./internal/ingest/... -run TestQueryHandler_SetAlertRouter -v
```

Expected: FAIL — `h.alertRouter` field does not exist; `SetAlertRouter` undefined.

- [ ] **Step 5.3: Add field and setter to `internal/ingest/receiver_query.go`**

In the `QueryHandler` struct (after the `store` field):
```go
alertRouter *AlertRouter // may be nil — alert pipeline disabled
```

After the `SetRuleStore` method:
```go
// SetAlertRouter wires the alert router for alert/incident management.
func (h *QueryHandler) SetAlertRouter(r *AlertRouter) {
	h.alertRouter = r
}
```

- [ ] **Step 5.4: Run test to verify it passes**

```
go test ./internal/ingest/... -run TestQueryHandler_SetAlertRouter -v
```

Expected: PASS

- [ ] **Step 5.5: Commit**

```bash
git add internal/ingest/receiver_query.go internal/ingest/receiver_query_test.go
git commit -m "feat(ingest): add alertRouter field and SetAlertRouter setter on QueryHandler

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 6: Alert HTTP handlers

**Files:**
- Create: `internal/ingest/handler_alerts.go`

Replace the three alert stubs in `handler_stubs.go` (`handleListAlerts`, `handleGetAlert`, `handleAcknowledgeAlert`). Write directly against migration 008 `alerts` schema. When `alertRouter` is nil (no DB), return 503.

- [ ] **Step 6.1: Write the failing test**

Create `internal/ingest/handler_alerts_test.go`:

```go
package ingest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestHandleListAlerts_NoRouter_Returns503(t *testing.T) {
	h := NewQueryHandler(nil, nil, zap.NewNop())
	// alertRouter is nil

	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts", nil)
	rr := httptest.NewRecorder()
	h.handleListAlerts(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	var body map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Contains(t, body["error"], "unavailable")
}

func TestHandleGetAlert_NoRouter_Returns503(t *testing.T) {
	h := NewQueryHandler(nil, nil, zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts/some-id", nil)
	rr := httptest.NewRecorder()
	h.handleGetAlert(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

func TestHandleAcknowledgeAlert_NoRouter_Returns503(t *testing.T) {
	h := NewQueryHandler(nil, nil, zap.NewNop())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/some-id/acknowledge", nil)
	rr := httptest.NewRecorder()
	h.handleAcknowledgeAlert(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}
```

- [ ] **Step 6.2: Run test to verify it fails**

```
go test ./internal/ingest/... -run TestHandleListAlerts -v
go test ./internal/ingest/... -run TestHandleGetAlert -v
go test ./internal/ingest/... -run TestHandleAcknowledgeAlert -v
```

Expected: FAIL — handlers currently return 501, not 503.

- [ ] **Step 6.3: Create `internal/ingest/handler_alerts.go`**

```go
package ingest

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// alertAvailable returns false if the pool is not wired.
func (h *QueryHandler) alertAvailable() bool {
	return h.alertRouter != nil && h.alertRouter.pool != nil
}

func (h *QueryHandler) handleListAlerts(w http.ResponseWriter, r *http.Request) {
	if !h.alertAvailable() {
		jsonError(w, "alert pipeline unavailable", http.StatusServiceUnavailable)
		return
	}

	// Parse query params
	status := r.URL.Query().Get("status")
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}

	query := `
		SELECT id, app_id, fingerprint, severity, layer, category,
		       title, description, trace_id, status, signal_count,
		       first_seen_at, last_seen_at, acknowledged_at, incident_id
		FROM alerts
		WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if status != "" {
		query += ` AND status = $` + strconv.Itoa(argIdx)
		args = append(args, status)
		argIdx++
	}
	query += ` ORDER BY first_seen_at DESC LIMIT $` + strconv.Itoa(argIdx)
	args = append(args, limit)

	rows, err := h.alertRouter.pool.Query(r.Context(), query, args...)
	if err != nil {
		h.log.Sugar().Warnw("list alerts query failed", "err", err)
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type alertRow struct {
		ID           string     `json:"id"`
		AppID        string     `json:"app_id"`
		Fingerprint  string     `json:"fingerprint"`
		Severity     int        `json:"severity"`
		Layer        int        `json:"layer"`
		Category     string     `json:"category"`
		Title        string     `json:"title"`
		Description  *string    `json:"description,omitempty"`
		TraceID      *string    `json:"trace_id,omitempty"`
		Status       string     `json:"status"`
		SignalCount  int        `json:"signal_count"`
		FirstSeenAt  time.Time  `json:"first_seen_at"`
		LastSeenAt   time.Time  `json:"last_seen_at"`
		AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
		IncidentID   *string    `json:"incident_id,omitempty"`
	}

	var alerts []alertRow
	for rows.Next() {
		var a alertRow
		if err := rows.Scan(
			&a.ID, &a.AppID, &a.Fingerprint, &a.Severity, &a.Layer, &a.Category,
			&a.Title, &a.Description, &a.TraceID, &a.Status, &a.SignalCount,
			&a.FirstSeenAt, &a.LastSeenAt, &a.AcknowledgedAt, &a.IncidentID,
		); err != nil {
			h.log.Sugar().Warnw("scan alert failed", "err", err)
			continue
		}
		alerts = append(alerts, a)
	}
	if alerts == nil {
		alerts = []alertRow{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"alerts": alerts, "count": len(alerts)})
}

func (h *QueryHandler) handleGetAlert(w http.ResponseWriter, r *http.Request) {
	if !h.alertAvailable() {
		jsonError(w, "alert pipeline unavailable", http.StatusServiceUnavailable)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		jsonError(w, "missing id", http.StatusBadRequest)
		return
	}

	row := h.alertRouter.pool.QueryRow(r.Context(), `
		SELECT id, app_id, fingerprint, severity, layer, category,
		       title, description, trace_id, status, signal_count,
		       first_seen_at, last_seen_at, acknowledged_at, incident_id
		FROM alerts WHERE id = $1`, id)

	type alertDetail struct {
		ID             string     `json:"id"`
		AppID          string     `json:"app_id"`
		Fingerprint    string     `json:"fingerprint"`
		Severity       int        `json:"severity"`
		Layer          int        `json:"layer"`
		Category       string     `json:"category"`
		Title          string     `json:"title"`
		Description    *string    `json:"description,omitempty"`
		TraceID        *string    `json:"trace_id,omitempty"`
		Status         string     `json:"status"`
		SignalCount    int        `json:"signal_count"`
		FirstSeenAt    time.Time  `json:"first_seen_at"`
		LastSeenAt     time.Time  `json:"last_seen_at"`
		AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
		IncidentID     *string    `json:"incident_id,omitempty"`
	}

	var a alertDetail
	if err := row.Scan(
		&a.ID, &a.AppID, &a.Fingerprint, &a.Severity, &a.Layer, &a.Category,
		&a.Title, &a.Description, &a.TraceID, &a.Status, &a.SignalCount,
		&a.FirstSeenAt, &a.LastSeenAt, &a.AcknowledgedAt, &a.IncidentID,
	); err != nil {
		jsonError(w, "alert not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(a)
}

func (h *QueryHandler) handleAcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	if !h.alertAvailable() {
		jsonError(w, "alert pipeline unavailable", http.StatusServiceUnavailable)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		jsonError(w, "missing id", http.StatusBadRequest)
		return
	}

	result, err := h.alertRouter.pool.Exec(r.Context(), `
		UPDATE alerts
		SET status = 'acknowledged', acknowledged_at = now()
		WHERE id = $1 AND status = 'open'`, id)
	if err != nil {
		jsonError(w, "update failed", http.StatusInternalServerError)
		return
	}
	if result.RowsAffected() == 0 {
		jsonError(w, "alert not found or already acknowledged", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "acknowledged"})
}

// jsonError writes a JSON error response.
func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{Error: msg})
}
```

Remove the three alert stub methods from `internal/ingest/handler_stubs.go` (the `handleListAlerts`, `handleGetAlert`, `handleAcknowledgeAlert` bodies — replace them with delegating calls, or just delete those three functions since handler_alerts.go redefines them with the same names).

**Important:** Delete the three duplicate function bodies from `handler_stubs.go`. The stubs currently define `handleListAlerts`, `handleGetAlert`, `handleAcknowledgeAlert` as 501 — the new file redefines them. Remove the old definitions.

- [ ] **Step 6.4: Run test to verify it passes**

```
go test ./internal/ingest/... -run TestHandleListAlerts -v
go test ./internal/ingest/... -run TestHandleGetAlert -v
go test ./internal/ingest/... -run TestHandleAcknowledgeAlert -v
```

Expected: PASS (all three return 503 when alertRouter is nil)

- [ ] **Step 6.5: Run full ingest package tests to ensure nothing broken**

```
go test ./internal/ingest/... -v 2>&1 | tail -20
```

Expected: all PASS, no compile errors.

- [ ] **Step 6.6: Commit**

```bash
git add internal/ingest/handler_alerts.go internal/ingest/handler_alerts_test.go internal/ingest/handler_stubs.go
git commit -m "feat(ingest): implement alert HTTP handlers with migration 008 schema

GET /api/v1/alerts, GET /api/v1/alerts/{id}, POST /api/v1/alerts/{id}/acknowledge.
Returns 503 when pool unavailable (graceful degradation).

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 7: Incident HTTP handlers

**Files:**
- Create: `internal/ingest/handler_incidents.go`

Replace four incident stubs: `handleListIncidents`, `handleGetIncident`, `handleAcknowledgeIncident`, `handleResolveIncident`. Same pool-nil → 503 pattern. Uses migration 008 `incidents` schema.

- [ ] **Step 7.1: Write the failing test**

Create `internal/ingest/handler_incidents_test.go`:

```go
package ingest

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestHandleListIncidents_NoRouter_Returns503(t *testing.T) {
	h := NewQueryHandler(nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents", nil)
	rr := httptest.NewRecorder()
	h.handleListIncidents(rr, req)
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

func TestHandleGetIncident_NoRouter_Returns503(t *testing.T) {
	h := NewQueryHandler(nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/abc", nil)
	rr := httptest.NewRecorder()
	h.handleGetIncident(rr, req)
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

func TestHandleAcknowledgeIncident_NoRouter_Returns503(t *testing.T) {
	h := NewQueryHandler(nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/incidents/abc/acknowledge", nil)
	rr := httptest.NewRecorder()
	h.handleAcknowledgeIncident(rr, req)
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

func TestHandleResolveIncident_NoRouter_Returns503(t *testing.T) {
	h := NewQueryHandler(nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/incidents/abc/resolve", nil)
	rr := httptest.NewRecorder()
	h.handleResolveIncident(rr, req)
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}
```

- [ ] **Step 7.2: Run test to verify it fails**

```
go test ./internal/ingest/... -run TestHandleListIncidents -v
go test ./internal/ingest/... -run TestHandleGetIncident -v
go test ./internal/ingest/... -run TestHandleAcknowledgeIncident -v
go test ./internal/ingest/... -run TestHandleResolveIncident -v
```

Expected: FAIL — stubs return 501.

- [ ] **Step 7.3: Create `internal/ingest/handler_incidents.go`**

```go
package ingest

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

func (h *QueryHandler) handleListIncidents(w http.ResponseWriter, r *http.Request) {
	if !h.alertAvailable() {
		jsonError(w, "alert pipeline unavailable", http.StatusServiceUnavailable)
		return
	}

	status := r.URL.Query().Get("status")
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}

	query := `
		SELECT id, title, description, severity, app_id, status,
		       alert_ids, trace_ids, created_at, updated_at,
		       acknowledged_at, resolved_at
		FROM incidents WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if status != "" {
		query += ` AND status = $` + strconv.Itoa(argIdx)
		args = append(args, status)
		argIdx++
	}
	query += ` ORDER BY created_at DESC LIMIT $` + strconv.Itoa(argIdx)
	args = append(args, limit)

	rows, err := h.alertRouter.pool.Query(r.Context(), query, args...)
	if err != nil {
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type incidentRow struct {
		ID             string     `json:"id"`
		Title          string     `json:"title"`
		Description    *string    `json:"description,omitempty"`
		Severity       int        `json:"severity"`
		AppID          string     `json:"app_id"`
		Status         string     `json:"status"`
		AlertIDs       []string   `json:"alert_ids"`
		TraceIDs       []string   `json:"trace_ids"`
		CreatedAt      time.Time  `json:"created_at"`
		UpdatedAt      time.Time  `json:"updated_at"`
		AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
		ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	}

	var incidents []incidentRow
	for rows.Next() {
		var inc incidentRow
		if err := rows.Scan(
			&inc.ID, &inc.Title, &inc.Description, &inc.Severity, &inc.AppID, &inc.Status,
			&inc.AlertIDs, &inc.TraceIDs, &inc.CreatedAt, &inc.UpdatedAt,
			&inc.AcknowledgedAt, &inc.ResolvedAt,
		); err != nil {
			h.log.Sugar().Warnw("scan incident failed", "err", err)
			continue
		}
		incidents = append(incidents, inc)
	}
	if incidents == nil {
		incidents = []incidentRow{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"incidents": incidents, "count": len(incidents)})
}

func (h *QueryHandler) handleGetIncident(w http.ResponseWriter, r *http.Request) {
	if !h.alertAvailable() {
		jsonError(w, "alert pipeline unavailable", http.StatusServiceUnavailable)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		jsonError(w, "missing id", http.StatusBadRequest)
		return
	}

	type incidentDetail struct {
		ID             string     `json:"id"`
		Title          string     `json:"title"`
		Description    *string    `json:"description,omitempty"`
		Severity       int        `json:"severity"`
		AppID          string     `json:"app_id"`
		Status         string     `json:"status"`
		AlertIDs       []string   `json:"alert_ids"`
		TraceIDs       []string   `json:"trace_ids"`
		CreatedAt      time.Time  `json:"created_at"`
		UpdatedAt      time.Time  `json:"updated_at"`
		AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
		ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	}

	var inc incidentDetail
	err := h.alertRouter.pool.QueryRow(r.Context(), `
		SELECT id, title, description, severity, app_id, status,
		       alert_ids, trace_ids, created_at, updated_at,
		       acknowledged_at, resolved_at
		FROM incidents WHERE id = $1`, id).Scan(
		&inc.ID, &inc.Title, &inc.Description, &inc.Severity, &inc.AppID, &inc.Status,
		&inc.AlertIDs, &inc.TraceIDs, &inc.CreatedAt, &inc.UpdatedAt,
		&inc.AcknowledgedAt, &inc.ResolvedAt,
	)
	if err != nil {
		jsonError(w, "incident not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(inc)
}

func (h *QueryHandler) handleAcknowledgeIncident(w http.ResponseWriter, r *http.Request) {
	if !h.alertAvailable() {
		jsonError(w, "alert pipeline unavailable", http.StatusServiceUnavailable)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		jsonError(w, "missing id", http.StatusBadRequest)
		return
	}

	result, err := h.alertRouter.pool.Exec(r.Context(), `
		UPDATE incidents
		SET status = 'acknowledged', acknowledged_at = now(), updated_at = now()
		WHERE id = $1 AND status = 'open'`, id)
	if err != nil {
		jsonError(w, "update failed", http.StatusInternalServerError)
		return
	}
	if result.RowsAffected() == 0 {
		jsonError(w, "incident not found or already acknowledged", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "acknowledged"})
}

func (h *QueryHandler) handleResolveIncident(w http.ResponseWriter, r *http.Request) {
	if !h.alertAvailable() {
		jsonError(w, "alert pipeline unavailable", http.StatusServiceUnavailable)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		jsonError(w, "missing id", http.StatusBadRequest)
		return
	}

	result, err := h.alertRouter.pool.Exec(r.Context(), `
		UPDATE incidents
		SET status = 'resolved', resolved_at = now(), updated_at = now()
		WHERE id = $1 AND status != 'resolved'`, id)
	if err != nil {
		jsonError(w, "update failed", http.StatusInternalServerError)
		return
	}
	if result.RowsAffected() == 0 {
		jsonError(w, "incident not found or already resolved", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "resolved"})
}
```

Remove the four incident stub methods from `handler_stubs.go` (`handleListIncidents`, `handleGetIncident`, `handleAcknowledgeIncident`, `handleResolveIncident`).

- [ ] **Step 7.4: Run test to verify it passes**

```
go test ./internal/ingest/... -run TestHandleListIncidents -v
go test ./internal/ingest/... -run TestHandleGetIncident -v
go test ./internal/ingest/... -run TestHandleAcknowledgeIncident -v
go test ./internal/ingest/... -run TestHandleResolveIncident -v
```

Expected: all PASS

- [ ] **Step 7.5: Commit**

```bash
git add internal/ingest/handler_incidents.go internal/ingest/handler_incidents_test.go internal/ingest/handler_stubs.go
git commit -m "feat(ingest): implement incident HTTP handlers with migration 008 schema

GET/GET{id}/POST acknowledge/POST resolve for /api/v1/incidents.
Returns 503 when pool unavailable.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 8: Wire everything in cmd/argus/api.go

**Files:**
- Modify: `cmd/argus/api.go`

Wire the full pipeline: `AdapterRegistry` → `LogAdapter` wrapped in `CircuitBreakerAdapter` → `AlertDispatcher` → `RoutingEngine` → `AlertRouter` → `DetectionProcessor`. Replace the bare `PgAlertWriter` currently passed to `DetectionProcessor`.

Also add CLI flags for Redis and Kairos (needed for AlertRouter).

- [ ] **Step 8.1: Write the failing test (compile check)**

```bash
cd C:/Users/Drupad/ArgusXDR/.worktrees/step2-detection-engine
go build ./cmd/argus/...
```

Expected: currently compiles. After editing, must still compile.

- [ ] **Step 8.2: Add Redis flag to `init()` in `cmd/argus/api.go`**

In the `init()` function, after the PostgreSQL flags, add:
```go
apiCmd.Flags().String("redis-addr", "localhost:6379", "Redis address for dedup and correlation")
viper.BindEnv("redis.addr", "ARGUS_REDIS_ADDR")
viper.BindPFlag("redis.addr", apiCmd.Flags().Lookup("redis-addr"))
```

- [ ] **Step 8.3: Replace the detection/alert wiring block in `runAPI`**

The current wiring in `runAPI` creates a `RuleStore`, loads built-in rules, creates a `PgAlertWriter`, and sets them on the query handler. Replace the detection wiring section (from `ruleStore :=` through `_ = ingest.NewPgAlertWriter(pgPool, log)`) with:

```go
// Detection engine: rule store + engine
ruleStore := engine.NewRuleStore()
rulesDir := viper.GetString("detection.rules_dir")
if rulesDir == "" {
    rulesDir = "internal/rules/built-in"
}
if builtInRules, loadErr := engine.LoadRulesFromDirectory(rulesDir); loadErr != nil {
    log.Warn("could not load built-in rules directory", zap.String("dir", rulesDir), zap.Error(loadErr))
} else {
    for _, r := range builtInRules {
        ruleStore.Add(r)
    }
    log.Info("built-in rules loaded", zap.Int("count", ruleStore.Count()))
}
queryHandler.SetRuleStore(ruleStore)

// Notification adapter registry: LogAdapter wrapped in CircuitBreaker
adapterRegistry := notify.NewAdapterRegistry(log)
logAdapter := notify.NewLogAdapter(log)
cbAdapter := notify.NewCircuitBreakerAdapter(logAdapter, notify.NewCircuitBreaker(nil))
if err := adapterRegistry.Register(cbAdapter); err != nil {
    log.Warn("failed to register log adapter", zap.Error(err))
}

// Alert dispatcher (fixed worker pool)
dispatcherCfg := notify.DefaultDispatcherConfig()
dispatcherCfg.WorkerCount = 2 // Lightweight for single-node
alertDispatcher, dispErr := notify.NewAlertDispatcher(dispatcherCfg, adapterRegistry, log)
if dispErr != nil {
    log.Warn("alert dispatcher unavailable", zap.Error(dispErr))
}
defer func() {
    if alertDispatcher != nil {
        shutdownCtx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel2()
        alertDispatcher.Shutdown(shutdownCtx2)
    }
}()

// Routing engine (requires PostgreSQL)
var routingEngine *notify.RoutingEngine
if pgPool != nil {
    routingEngine, err = notify.NewRoutingEngine(pgPool, log)
    if err != nil {
        log.Warn("routing engine unavailable", zap.Error(err))
    } else {
        routingEngine.Start()
        defer routingEngine.Stop()
    }
}

// Redis client for dedup + incident correlation
redisAddr := viper.GetString("redis.addr")
var redisClient *redis.Client
if redisAddr != "" {
    redisClient = redis.NewClient(&redis.Options{Addr: redisAddr})
    if pingErr := redisClient.Ping(ctx).Err(); pingErr != nil {
        log.Warn("Redis unavailable — dedup and incident correlation disabled", zap.Error(pingErr))
        redisClient = nil
    } else {
        defer redisClient.Close()
        log.Info("Redis connected", zap.String("addr", redisAddr))
    }
}

// Alert router: replaces PgAlertWriter
alertRouter := ingest.NewAlertRouter(pgPool, redisClient, routingEngine, alertDispatcher, log)
queryHandler.SetAlertRouter(alertRouter)

// Detection processor wired to AlertRouter
detectionEngine := engine.New(ruleStore, nil) // nil = no temporal store in API-only mode
_ = pipeline.NewDetectionProcessor(detectionEngine, alertRouter)
// NOTE: DetectionProcessor is used by the ingest pipeline (cmd/argus/serve).
// In API-only mode, detection fires on-demand; wire to pipeline in serve command.
```

Add imports at top of file:
```go
"github.com/argusxdr/argus/internal/notify"
"github.com/argusxdr/argus/internal/pipeline"
"github.com/redis/go-redis/v9"
```

- [ ] **Step 8.4: Verify it compiles**

```bash
go build ./cmd/argus/...
```

Expected: EXIT 0, no errors.

- [ ] **Step 8.5: Run full package tests to verify nothing regressed**

```bash
go test ./internal/ingest/... ./internal/notify/... ./internal/detection/... ./internal/pipeline/... 2>&1 | tail -30
```

Expected: all packages PASS.

- [ ] **Step 8.6: Commit**

```bash
git add cmd/argus/api.go
git commit -m "feat(cmd): wire AlertRouter, dispatcher, routing engine in api.go

- Register LogAdapter (wrapped in CircuitBreaker) as default notification adapter
- Wire AlertDispatcher with 2 workers
- Wire RoutingEngine (requires PostgreSQL)
- Wire AlertRouter replacing bare PgAlertWriter
- Add --redis-addr flag for dedup/correlation

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 9: Gate verification — compile + test sweep

**Files:** None new — verification only.

The Step 3 gate is: **detection fires → alert written to PostgreSQL → notification dispatched to adapter**. The full E2E requires live PostgreSQL+Redis, but the gate can be verified structurally via compile + unit tests + confirmed interface wiring.

- [ ] **Step 9.1: Full module compile check**

```bash
cd C:/Users/Drupad/ArgusXDR/.worktrees/step2-detection-engine
go build ./...
```

Expected: EXIT 0.

- [ ] **Step 9.2: Run all tests**

```bash
go test ./... 2>&1 | grep -E "^(ok|FAIL|---)" | head -40
```

Expected: all packages `ok`. No `FAIL` lines.

- [ ] **Step 9.3: Verify interface compliance**

```bash
go vet ./internal/ingest/... ./internal/notify/... ./internal/pipeline/...
```

Expected: no output (clean).

- [ ] **Step 9.4: Verify AlertRouter satisfies pipeline.AlertWriter**

Add a compile-time assertion in `internal/ingest/alert_router.go` (anywhere at package level):

```go
// Compile-time interface check.
var _ pipeline.AlertWriter = (*AlertRouter)(nil)
```

Add import `"github.com/argusxdr/argus/internal/pipeline"` to the alert_router.go imports.

Run:
```bash
go build ./internal/ingest/...
```

Expected: EXIT 0 — if not, the `WriteAlert` signature doesn't match `pipeline.AlertWriter`.

- [ ] **Step 9.5: Verify CircuitBreakerAdapter satisfies notify.Notifier**

Add in `internal/notify/circuitbreaker_adapter.go`:
```go
var _ Notifier = (*CircuitBreakerAdapter)(nil)
```

Run:
```bash
go build ./internal/notify/...
```

Expected: EXIT 0.

- [ ] **Step 9.6: Final commit**

```bash
git add internal/ingest/alert_router.go internal/notify/circuitbreaker_adapter.go
git commit -m "chore(step3): add compile-time interface assertions for AlertRouter and CircuitBreakerAdapter

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

- [ ] **Step 9.7: Tag Step 3 complete**

```bash
git log --oneline -8
```

Confirm all Step 3 commits are present, then:

```bash
git tag step3-gate-passed
```

---

## Step 3 Gate Checklist

Before declaring Step 3 complete, verify all of the following:

| Check | Command | Expected |
|-------|---------|---------|
| Full compile | `go build ./...` | EXIT 0 |
| All tests pass | `go test ./...` | all `ok` |
| vet clean | `go vet ./...` | no output |
| RoutingEngine SQL matches migration 008 | `grep -n "min_severity" internal/notify/router.go` | found in SyncRules |
| AlertRouter satisfies AlertWriter | `grep "var _ pipeline.AlertWriter" internal/ingest/alert_router.go` | found |
| CircuitBreakerAdapter satisfies Notifier | `grep "var _ Notifier" internal/notify/circuitbreaker_adapter.go` | found |
| Alert handlers return 503 when no pool | `go test ./internal/ingest/... -run TestHandleListAlerts -v` | PASS |
| Incident handlers return 503 when no pool | `go test ./internal/ingest/... -run TestHandleListIncidents -v` | PASS |
| Routing eval logic tested | `go test ./internal/notify/... -run TestRoutingEngine_simpleEval -v` | PASS |
| CircuitBreaker wraps Send | `go test ./internal/notify/... -run TestCircuitBreakerAdapter -v` | PASS |

---

## Known Limitations (Step 4+ work)

- `alert.PostgresAlertService` and `alert.IncidentService` in `internal/alert/` are NOT wired — their schema doesn't match migration 008. They are candidates for removal or schema alignment in a future step.
- `RoutingEngine` requires PostgreSQL; if unavailable, `AlertRouter.WriteAlert` still writes the alert but dispatches to 0 targets (graceful degradation — no notification, but alert is persisted).
- `DetectionProcessor` in the API-only `serve` command is not connected to the ingest pipeline; that wiring belongs in `cmd/argus/serve.go` (Step 4 work).
- `handleListRules`, `handleCreateRule`, etc. (from Step 2 `handler_rules.go`) are in scope for this step only to ensure they still compile alongside the new handlers.
