# Argus XDR v1.0 — Step 1: Data Fabric Fix

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix all broken infrastructure, implement every missing API endpoint (Tier 1–3), add WebSocket signal streaming, and ensure every route returns non-404.

**Architecture:** Chi router hosts all routes on a single mux; handlers check ClickHouse/PostgreSQL availability and return 503 when degraded (P3). `RegisterRoutes()` in each handler type adds routes to the shared mux. WebSocket uses a pipeline `SignalBroadcaster` fan-out channel per subscriber.

**Tech Stack:** Go 1.22, chi v5.0, gorilla/websocket or stdlib WebSocket upgrade, ClickHouse clickhouse-go/v2, PostgreSQL pgx/v5, Redis go-redis/v9, Prometheus client_golang v1.19+

---

## Pre-Work: Root Cause Confirmed

The "chi routing bug" was a **stale process shadowing** issue:
- `api.go` logged "HTTP server listening" BEFORE calling `ListenAndServe()`
- If port 8080 was already bound by a stale binary, the new process silently failed in its goroutine
- Old process (without new routes) continued answering requests
- **Fixed:** `net.Listen()` now called before the log line; server uses `Serve(ln)` not `ListenAndServe()`

The chi router itself was never broken.

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `cmd/argus/api.go` | ✓ Modified | API command, graceful ClickHouse startup, component health endpoint |
| `internal/ingest/receiver_query.go` | Extend | All Tier 1 query handlers (signals, schema, layers, traces, SQL query) |
| `internal/ingest/receiver_query_test.go` | Create | Route registration test + handler tests |
| `internal/ingest/receiver_ws.go` | Create | WebSocket handler + per-client send buffer |
| `internal/ingest/broadcaster.go` | Create | Pipeline SignalBroadcaster (fan-out channel) |
| `internal/ingest/handler_alerts.go` | Create | Tier 2: /api/v1/alerts CRUD |
| `internal/ingest/handler_incidents.go` | Create | Tier 2: /api/v1/incidents CRUD |
| `internal/ingest/handler_rules.go` | Create | Tier 2: /api/v1/rules CRUD |
| `internal/ingest/handler_auth.go` | Create | Tier 3: /api/v1/auth/* endpoints |
| `internal/ingest/handler_users.go` | Create | Tier 3: /api/v1/users CRUD |
| `internal/ingest/handler_apps.go` | Create | Tier 3: /api/v1/apps CRUD |
| `internal/storage/clickhouse.go` | ✓ Extended | Added Ping() method |
| `migrations/008_core_tables.up.sql` | Create | apps, detection_rules, alerts, incidents, notification_channels tables |
| `migrations/008_core_tables.down.sql` | Create | Rollback |

---

## Task 1: Route Registration Test

**Files:**
- Create: `internal/ingest/receiver_query_test.go`

This test is permanent. It catches route regressions forever. Run it after every change to routes.

- [ ] **Step 1.1: Write the route coverage test**

```go
package ingest_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/argusxdr/argus/internal/ingest"
	"github.com/argusxdr/argus/internal/metrics"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestRouteRegistration verifies every registered path returns non-404.
// This is a permanent regression guard — a 404 means a route was removed or renamed.
func TestRouteRegistration(t *testing.T) {
	mux := chi.NewRouter()
	qh := ingest.NewQueryHandler(nil, &metrics.HTTP{}, zap.NewNop())
	qh.RegisterRoutes(mux)

	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/v1/signals"},
		{"GET", "/v1/schema/signals"},
		{"GET", "/api/v1/layers/status"},
		{"GET", "/api/v1/traces/test-trace-id"},
		{"POST", "/api/v1/query"},
		{"GET", "/api/v1/rules"},
		{"POST", "/api/v1/rules"},
		{"GET", "/api/v1/alerts"},
		{"GET", "/api/v1/incidents"},
		{"POST", "/api/v1/auth/login"},
		{"POST", "/api/v1/auth/refresh"},
		{"POST", "/api/v1/auth/logout"},
		{"GET", "/api/v1/users"},
		{"GET", "/api/v1/apps"},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			req := httptest.NewRequest(route.method, route.path, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			assert.NotEqual(t, 404, w.Code,
				"route %s %s returned 404 — is it registered in RegisterRoutes()?",
				route.method, route.path)
		})
	}
}
```

- [ ] **Step 1.2: Run test to see which routes fail (they will — stubs not yet added)**

```
cd C:/Users/Drupad/ArgusXDR && go test ./internal/ingest/... -run TestRouteRegistration -v 2>&1
```

Expected: FAIL on missing routes (rules, alerts, incidents, auth, users, apps)

- [ ] **Step 1.3: Add stub registrations in RegisterRoutes()**

In `internal/ingest/receiver_query.go`, update `RegisterRoutes`:

```go
func (h *QueryHandler) RegisterRoutes(mux *chi.Mux) {
	// Tier 1 — Dashboard fundamentals
	mux.Get("/v1/signals", h.handleGetSignals)
	mux.Get("/v1/schema/signals", h.HandleGetSignalSchema)
	mux.Get("/api/v1/layers/status", h.handleGetLayerStatus)
	mux.Get("/api/v1/traces/{traceId}", h.handleGetTrace)
	mux.Post("/api/v1/query", h.handlePostQuery)

	// Tier 2 — Detection & response (stubs return 501)
	mux.Get("/api/v1/rules", h.handleListRules)
	mux.Post("/api/v1/rules", h.handleCreateRule)
	mux.Get("/api/v1/rules/{id}", h.handleGetRule)
	mux.Put("/api/v1/rules/{id}", h.handleUpdateRule)
	mux.Delete("/api/v1/rules/{id}", h.handleDeleteRule)
	mux.Post("/api/v1/rules/validate", h.handleValidateRule)
	mux.Post("/api/v1/rules/test", h.handleTestRule)
	mux.Get("/api/v1/alerts", h.handleListAlerts)
	mux.Get("/api/v1/alerts/{id}", h.handleGetAlert)
	mux.Post("/api/v1/alerts/{id}/acknowledge", h.handleAcknowledgeAlert)
	mux.Get("/api/v1/incidents", h.handleListIncidents)
	mux.Get("/api/v1/incidents/{id}", h.handleGetIncident)
	mux.Post("/api/v1/incidents/{id}/acknowledge", h.handleAcknowledgeIncident)
	mux.Post("/api/v1/incidents/{id}/resolve", h.handleResolveIncident)

	// Tier 3 — Auth & management (stubs return 501)
	mux.Post("/api/v1/auth/login", h.handleLogin)
	mux.Post("/api/v1/auth/refresh", h.handleRefreshToken)
	mux.Post("/api/v1/auth/logout", h.handleLogout)
	mux.Post("/api/v1/auth/setup", h.handleSetup)
	mux.Get("/api/v1/users", h.handleListUsers)
	mux.Post("/api/v1/users", h.handleCreateUser)
	mux.Get("/api/v1/apps", h.handleListApps)
	mux.Post("/api/v1/apps", h.handleCreateApp)
	mux.Get("/api/v1/apps/{id}/key", h.handleGetAppKey)
	mux.Post("/api/v1/apps/{id}/key/rotate", h.handleRotateAppKey)
	mux.Get("/api/v1/audit", h.handleListAuditLog)
}
```

All stub handlers follow this pattern (add to the end of `receiver_query.go`):

```go
func (h *QueryHandler) handleListRules(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(ErrorResponse{Error: "not implemented"})
}
// ... repeat for every stub handler listed above
```

- [ ] **Step 1.4: Run test to verify all routes return non-404**

```
cd C:/Users/Drupad/ArgusXDR && go test ./internal/ingest/... -run TestRouteRegistration -v 2>&1
```

Expected: All PASS (501 is acceptable, 404 is not)

- [ ] **Step 1.5: Commit**

```bash
git add internal/ingest/receiver_query.go internal/ingest/receiver_query_test.go
git commit -m "feat: register all API routes (Tier 1-3 stubs), fix route coverage test"
```

---

## Task 2: PostgreSQL Core Tables Migration

**Files:**
- Create: `migrations/008_core_tables.up.sql`
- Create: `migrations/008_core_tables.down.sql`

- [ ] **Step 2.1: Write the migration**

Create `migrations/008_core_tables.up.sql`:

```sql
-- Apps: registered applications with API keys
CREATE TABLE IF NOT EXISTS apps (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    description TEXT,
    api_key     TEXT NOT NULL UNIQUE,           -- hashed with SHA256
    api_key_prefix TEXT NOT NULL,               -- first 8 chars for display
    created_by  UUID REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    status      TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked'))
);
CREATE INDEX IF NOT EXISTS idx_apps_api_key ON apps(api_key);
CREATE INDEX IF NOT EXISTS idx_apps_status ON apps(status);

-- Detection rules
CREATE TABLE IF NOT EXISTS detection_rules (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    description TEXT,
    tier        INTEGER NOT NULL CHECK (tier IN (1, 2, 3)),
    enabled     BOOLEAN NOT NULL DEFAULT true,
    yaml_config JSONB NOT NULL,                 -- rule definition
    created_by  UUID REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_detection_rules_enabled ON detection_rules(enabled);
CREATE INDEX IF NOT EXISTS idx_detection_rules_tier ON detection_rules(tier);

-- Alerts
CREATE TABLE IF NOT EXISTS alerts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id         UUID REFERENCES detection_rules(id),
    app_id          TEXT NOT NULL,
    fingerprint     TEXT NOT NULL,              -- SHA256(rule_id+app_id+layer+category)
    severity        INTEGER NOT NULL CHECK (severity BETWEEN 1 AND 5),
    layer           INTEGER NOT NULL CHECK (layer BETWEEN 1 AND 10),
    category        TEXT NOT NULL,
    title           TEXT NOT NULL,
    description     TEXT,
    signal_ids      TEXT[] NOT NULL DEFAULT '{}',
    trace_id        TEXT,
    incident_id     UUID,
    status          TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'acknowledged', 'resolved', 'suppressed')),
    signal_count    INTEGER NOT NULL DEFAULT 1, -- incremented on dedup
    first_seen_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    acknowledged_at TIMESTAMPTZ,
    acknowledged_by UUID REFERENCES users(id),
    resolved_at     TIMESTAMPTZ,
    resolved_by     UUID REFERENCES users(id)
);
CREATE INDEX IF NOT EXISTS idx_alerts_fingerprint ON alerts(fingerprint);
CREATE INDEX IF NOT EXISTS idx_alerts_app_id ON alerts(app_id);
CREATE INDEX IF NOT EXISTS idx_alerts_status ON alerts(status);
CREATE INDEX IF NOT EXISTS idx_alerts_trace_id ON alerts(trace_id);
CREATE INDEX IF NOT EXISTS idx_alerts_incident_id ON alerts(incident_id);
CREATE INDEX IF NOT EXISTS idx_alerts_first_seen ON alerts(first_seen_at DESC);

-- Incidents
CREATE TABLE IF NOT EXISTS incidents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title           TEXT NOT NULL,
    description     TEXT,
    severity        INTEGER NOT NULL CHECK (severity BETWEEN 1 AND 5),
    app_id          TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'acknowledged', 'resolved')),
    alert_ids       UUID[] NOT NULL DEFAULT '{}',
    trace_ids       TEXT[] NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    acknowledged_at TIMESTAMPTZ,
    acknowledged_by UUID REFERENCES users(id),
    resolved_at     TIMESTAMPTZ,
    resolved_by     UUID REFERENCES users(id)
);
CREATE INDEX IF NOT EXISTS idx_incidents_app_id ON incidents(app_id);
CREATE INDEX IF NOT EXISTS idx_incidents_status ON incidents(status);
CREATE INDEX IF NOT EXISTS idx_incidents_created ON incidents(created_at DESC);

-- Notification channels
CREATE TABLE IF NOT EXISTS notification_channels (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    type        TEXT NOT NULL CHECK (type IN ('slack', 'email', 'pagerduty', 'webhook', 'syslog')),
    config      JSONB NOT NULL,                 -- adapter-specific config
    enabled     BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Alert routing rules (which alerts go to which channels)
CREATE TABLE IF NOT EXISTS routing_rules (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id      UUID NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    min_severity    INTEGER NOT NULL DEFAULT 1 CHECK (min_severity BETWEEN 1 AND 5),
    app_id_filter   TEXT,                       -- NULL = all apps
    layer_filter    INTEGER,                    -- NULL = all layers
    enabled         BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Suppression rules (dedup windows)
CREATE TABLE IF NOT EXISTS suppression_rules (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    fingerprint     TEXT NOT NULL,
    suppressed_until TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_suppression_fingerprint ON suppression_rules(fingerprint);
CREATE INDEX IF NOT EXISTS idx_suppression_until ON suppression_rules(suppressed_until);
```

Create `migrations/008_core_tables.down.sql`:

```sql
DROP TABLE IF EXISTS suppression_rules;
DROP TABLE IF EXISTS routing_rules;
DROP TABLE IF EXISTS notification_channels;
DROP TABLE IF EXISTS incidents;
DROP TABLE IF EXISTS alerts;
DROP TABLE IF EXISTS detection_rules;
DROP TABLE IF EXISTS apps;
```

- [ ] **Step 2.2: Verify migration syntax (no DB needed)**

```bash
grep -c "CREATE TABLE" migrations/008_core_tables.up.sql
# Expected: 7
```

- [ ] **Step 2.3: Commit**

```bash
git add migrations/008_core_tables.up.sql migrations/008_core_tables.down.sql
git commit -m "feat: add core tables migration (apps, rules, alerts, incidents, channels)"
```

---

## Task 3: Implement Tier 1 — `/api/v1/traces/{traceId}`

**Files:**
- Modify: `internal/ingest/receiver_query.go` — implement `handleGetTrace`

- [ ] **Step 3.1: Write the failing test**

In `internal/ingest/receiver_query_test.go`, add:

```go
func TestHandleGetTrace_NilStorage(t *testing.T) {
	mux := chi.NewRouter()
	qh := ingest.NewQueryHandler(nil, &metrics.HTTP{}, zap.NewNop())
	qh.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/api/v1/traces/trace-abc-123", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// With nil ClickHouse, should return 503 (not 404, not 500)
	assert.Equal(t, 503, w.Code)
	assert.Contains(t, w.Body.String(), "storage unavailable")
}
```

- [ ] **Step 3.2: Run test to verify it fails**

```bash
cd C:/Users/Drupad/ArgusXDR && go test ./internal/ingest/... -run TestHandleGetTrace -v 2>&1
```

Expected: FAIL — handler returns 501 not 503

- [ ] **Step 3.3: Implement `handleGetTrace`**

In `internal/ingest/receiver_query.go`, replace the stub:

```go
// TraceResponse is the JSON response for GET /api/v1/traces/{traceId}.
type TraceResponse struct {
	TraceID string            `json:"trace_id"`
	Signals []*v1.ArgusSignal `json:"signals"`
}

func (h *QueryHandler) handleGetTrace(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if h.ch == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "storage unavailable"})
		return
	}

	traceID := chi.URLParam(r, "traceId")
	if traceID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "traceId is required"})
		return
	}

	ctx := r.Context()
	query := buildSignalQuery("", 0, "", 0, nil, nil, nil, "", 1000)
	// Override with trace_id filter
	query = `
SELECT
	signal_id, trace_id, span_id, parent_span_id,
	layer, category, severity,
	timestamp, duration_ms, ingested_at,
	app_id, app_version, sdk_version, environment, host_id,
	provider_name, provider_model,
	related_signals, incident_id, session_id, conversation_id, user_id,
	data_classification, retention_policy, pii_detected,
	ctx_l1_cpu_percent, ctx_l1_memory_used_mb, ctx_l1_gpu_utilization_pct,
	ctx_l2_model_id, ctx_l2_model_hash, ctx_l2_quantization,
	ctx_l3_input_token_count, ctx_l3_output_token_count, ctx_l3_truncated,
	ctx_l4_attention_entropy, ctx_l4_kv_cache_hit_rate,
	ctx_l5_mean_logprob, ctx_l5_top_logprob, ctx_l5_finish_reason,
	ctx_l6_safety_score, ctx_l6_policy_violated, ctx_l6_action_taken,
	ctx_l7_query_text, ctx_l7_retrieved_count, ctx_l7_top_score, ctx_l7_collection_name,
	ctx_l8_tool_name, ctx_l8_tool_input_hash, ctx_l8_agent_step,
	ctx_l9_method, ctx_l9_path, ctx_l9_status_code, ctx_l9_latency_ms,
	ctx_l10_event_type, ctx_l10_component,
	enrich_baseline_deviation, enrich_geoip_country, enrich_geoip_city, enrich_threat_intel_hit
FROM signals FINAL
WHERE trace_id = {trace_id:String}
ORDER BY timestamp ASC
LIMIT 1000`

	rows, err := h.ch.Conn().Query(ctx, query, clickhouse.Named("trace_id", traceID))
	if err != nil {
		h.log.Error("trace query failed", zap.String("trace_id", traceID), zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "query failed"})
		return
	}
	defer rows.Close()

	signals, err := scanSignalRows(rows, 1000)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(TraceResponse{TraceID: traceID, Signals: signals})
}
```

Also extract the row scanning logic into a helper (DRY):

```go
// scanSignalRows scans ClickHouse rows into ArgusSignal slice.
// Extracted to avoid duplication between handleGetSignals and handleGetTrace.
func scanSignalRows(rows driver.Rows, limit int64) ([]*v1.ArgusSignal, error) {
	// (move the existing scan loop from handleGetSignals here)
	// Return signals slice and any row iteration error.
}
```

- [ ] **Step 3.4: Add clickhouse import to receiver_query.go**

At top of file, ensure import:
```go
import (
    "github.com/ClickHouse/clickhouse-go/v2"
    // ... existing imports
)
```

- [ ] **Step 3.5: Run tests**

```bash
cd C:/Users/Drupad/ArgusXDR && go test ./internal/ingest/... -v 2>&1
```

Expected: TestHandleGetTrace_NilStorage PASS, TestRouteRegistration PASS

- [ ] **Step 3.6: Commit**

```bash
git add internal/ingest/receiver_query.go internal/ingest/receiver_query_test.go
git commit -m "feat: implement GET /api/v1/traces/{traceId} with storage degradation"
```

---

## Task 4: Implement Tier 1 — `POST /api/v1/query` (Safe SQL)

**Files:**
- Modify: `internal/ingest/receiver_query.go` — implement `handlePostQuery`

- [ ] **Step 4.1: Write the failing test**

```go
func TestHandlePostQuery_BlocksDDL(t *testing.T) {
	mux := chi.NewRouter()
	qh := ingest.NewQueryHandler(nil, &metrics.HTTP{}, zap.NewNop())
	qh.RegisterRoutes(mux)

	// DDL should be blocked regardless of ClickHouse availability
	body := `{"sql":"DROP TABLE signals"}`
	req := httptest.NewRequest("POST", "/api/v1/query",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "DDL")
}

func TestHandlePostQuery_NilStorage(t *testing.T) {
	mux := chi.NewRouter()
	qh := ingest.NewQueryHandler(nil, &metrics.HTTP{}, zap.NewNop())
	qh.RegisterRoutes(mux)

	body := `{"sql":"SELECT count() FROM signals"}`
	req := httptest.NewRequest("POST", "/api/v1/query",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, 503, w.Code)
}
```

- [ ] **Step 4.2: Run test to see it fail**

```bash
cd C:/Users/Drupad/ArgusXDR && go test ./internal/ingest/... -run TestHandlePostQuery -v 2>&1
```

Expected: FAIL — stub returns 501

- [ ] **Step 4.3: Implement `handlePostQuery`**

```go
// QueryRequest is the JSON body for POST /api/v1/query.
type QueryRequest struct {
	SQL   string `json:"sql"`
	Limit int    `json:"limit"` // default 1000, max 5000
}

// QueryResultResponse is the JSON response for POST /api/v1/query.
type QueryResultResponse struct {
	Columns []string        `json:"columns"`
	Rows    [][]interface{} `json:"rows"`
	RowCount int            `json:"row_count"`
}

// ddlPatterns blocks destructive/mutating SQL
var ddlPatterns = []string{
	"DROP", "DELETE", "INSERT", "UPDATE", "ALTER", "TRUNCATE",
	"CREATE", "REPLACE", "RENAME", "OPTIMIZE", "SYSTEM",
}

func (h *QueryHandler) handlePostQuery(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if h.ch == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "storage unavailable"})
		return
	}

	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "invalid JSON body"})
		return
	}
	if req.SQL == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "sql is required"})
		return
	}

	// Safety: block DDL and mutating statements
	sqlUpper := strings.ToUpper(strings.TrimSpace(req.SQL))
	for _, pattern := range ddlPatterns {
		if strings.HasPrefix(sqlUpper, pattern) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{
				Error: fmt.Sprintf("DDL and mutating statements are not allowed: %s", pattern),
			})
			return
		}
	}

	// Enforce row limit (inject LIMIT if not present)
	limit := req.Limit
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	if !strings.Contains(sqlUpper, "LIMIT") {
		req.SQL = fmt.Sprintf("%s LIMIT %d", req.SQL, limit)
	}

	// Execute with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	rows, err := h.ch.Conn().Query(ctx, req.SQL)
	if err != nil {
		h.log.Warn("user query failed", zap.String("sql", req.SQL), zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
		return
	}
	defer rows.Close()

	// Get column names
	colTypes := rows.ColumnTypes()
	columns := make([]string, len(colTypes))
	for i, ct := range colTypes {
		columns[i] = ct.Name()
	}

	// Scan all rows
	var resultRows [][]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		ptrs := make([]interface{}, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}
		resultRows = append(resultRows, values)
	}
	if err := rows.Err(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "scan error: " + err.Error()})
		return
	}

	if resultRows == nil {
		resultRows = [][]interface{}{}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(QueryResultResponse{
		Columns:  columns,
		Rows:     resultRows,
		RowCount: len(resultRows),
	})
}
```

- [ ] **Step 4.4: Run tests**

```bash
cd C:/Users/Drupad/ArgusXDR && go test ./internal/ingest/... -run TestHandlePostQuery -v 2>&1
```

Expected: Both tests PASS

- [ ] **Step 4.5: Commit**

```bash
git add internal/ingest/receiver_query.go internal/ingest/receiver_query_test.go
git commit -m "feat: implement POST /api/v1/query with DDL safety and 30s timeout"
```

---

## Task 5: Signal Broadcaster (Pipeline Fan-Out)

**Files:**
- Create: `internal/ingest/broadcaster.go`
- Create: `internal/ingest/broadcaster_test.go`

This is the core mechanism for WebSocket streaming. The pipeline's final stage publishes signals here; WebSocket clients subscribe.

- [ ] **Step 5.1: Write the failing test**

Create `internal/ingest/broadcaster_test.go`:

```go
package ingest_test

import (
	"context"
	"testing"
	"time"

	"github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/ingest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBroadcaster_SubscribeReceivesSignals(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	b := ingest.NewSignalBroadcaster()
	go b.Run(ctx)

	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	sig := &v1.ArgusSignal{SignalId: "test-001", TraceId: "trace-001"}
	b.Publish(sig)

	select {
	case got := <-ch:
		assert.Equal(t, "test-001", got.SignalId)
	case <-time.After(time.Second):
		t.Fatal("timeout: signal not received")
	}
}

func TestBroadcaster_SlowSubscriberDropsOldest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	b := ingest.NewSignalBroadcaster()
	go b.Run(ctx)

	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	// Flood 200 signals into a buffer of 100
	for i := 0; i < 200; i++ {
		sig := &v1.ArgusSignal{SignalId: fmt.Sprintf("signal-%d", i)}
		b.Publish(sig)
	}

	// Subscriber should not block; we should read some signals
	received := 0
	timeout := time.After(500 * time.Millisecond)
	for {
		select {
		case <-ch:
			received++
		case <-timeout:
			goto done
		}
	}
done:
	assert.Greater(t, received, 0, "should receive at least some signals")
}

func TestBroadcaster_MultipleSubscribers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	b := ingest.NewSignalBroadcaster()
	go b.Run(ctx)

	ch1 := b.Subscribe()
	ch2 := b.Subscribe()
	defer b.Unsubscribe(ch1)
	defer b.Unsubscribe(ch2)

	sig := &v1.ArgusSignal{SignalId: "broadcast-001"}
	b.Publish(sig)

	for _, ch := range []<-chan *v1.ArgusSignal{ch1, ch2} {
		select {
		case got := <-ch:
			assert.Equal(t, "broadcast-001", got.SignalId)
		case <-time.After(time.Second):
			t.Fatal("timeout: subscriber did not receive signal")
		}
	}
}
```

- [ ] **Step 5.2: Run test to verify it fails**

```bash
cd C:/Users/Drupad/ArgusXDR && go test ./internal/ingest/... -run TestBroadcaster -v 2>&1
```

Expected: FAIL — SignalBroadcaster undefined

- [ ] **Step 5.3: Implement `SignalBroadcaster`**

Create `internal/ingest/broadcaster.go`:

```go
package ingest

import (
	"context"
	"sync"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
)

const subscriberBufferSize = 100 // per P2: per-client send buffer

// SignalBroadcaster fans out signals from the pipeline to all WebSocket subscribers.
// Per P2: slow subscribers get their oldest signal dropped (never block ingest).
type SignalBroadcaster struct {
	mu          sync.RWMutex
	subscribers map[chan *v1.ArgusSignal]struct{}
	publish     chan *v1.ArgusSignal
}

// NewSignalBroadcaster creates a new broadcaster. Call Run(ctx) in a goroutine.
func NewSignalBroadcaster() *SignalBroadcaster {
	return &SignalBroadcaster{
		subscribers: make(map[chan *v1.ArgusSignal]struct{}),
		publish:     make(chan *v1.ArgusSignal, 1000),
	}
}

// Run processes the publish channel, fanning out to all subscribers.
// Exits when ctx is cancelled.
func (b *SignalBroadcaster) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case sig := <-b.publish:
			b.fan(sig)
		}
	}
}

// fan delivers sig to all current subscribers.
// If a subscriber's buffer is full, drop the oldest signal (non-blocking send).
func (b *SignalBroadcaster) fan(sig *v1.ArgusSignal) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subscribers {
		select {
		case ch <- sig:
			// delivered
		default:
			// buffer full — drain one to make room (drop oldest), then try again
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- sig:
			default:
			}
		}
	}
}

// Publish sends a signal to all subscribers asynchronously.
// Never blocks — drops if publish channel is full.
func (b *SignalBroadcaster) Publish(sig *v1.ArgusSignal) {
	select {
	case b.publish <- sig:
	default:
		// publish buffer full — drop (P2: never block ingest hot path)
	}
}

// Subscribe creates a new per-client buffered channel.
func (b *SignalBroadcaster) Subscribe() chan *v1.ArgusSignal {
	ch := make(chan *v1.ArgusSignal, subscriberBufferSize)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber and closes its channel.
func (b *SignalBroadcaster) Unsubscribe(ch chan *v1.ArgusSignal) {
	b.mu.Lock()
	delete(b.subscribers, ch)
	b.mu.Unlock()
	close(ch)
}

// SubscriberCount returns current subscriber count (for metrics).
func (b *SignalBroadcaster) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}
```

- [ ] **Step 5.4: Run tests**

```bash
cd C:/Users/Drupad/ArgusXDR && go test ./internal/ingest/... -run TestBroadcaster -v 2>&1
```

Expected: All 3 broadcaster tests PASS

- [ ] **Step 5.5: Commit**

```bash
git add internal/ingest/broadcaster.go internal/ingest/broadcaster_test.go
git commit -m "feat: add SignalBroadcaster (fan-out, per-client buffer, drop-oldest on full)"
```

---

## Task 6: WebSocket Signal Stream

**Files:**
- Create: `internal/ingest/receiver_ws.go`
- Create: `internal/ingest/receiver_ws_test.go`
- Modify: `cmd/argus/api.go` — wire broadcaster to WS handler

- [ ] **Step 6.1: Add gorilla/websocket dependency**

```bash
cd C:/Users/Drupad/ArgusXDR && go get github.com/gorilla/websocket@v1.5.3 && go mod tidy
```

- [ ] **Step 6.2: Write the failing test**

Create `internal/ingest/receiver_ws_test.go`:

```go
package ingest_test

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/argusxdr/argus/internal/ingest"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebSocketHandler_ReceivesSignals(t *testing.T) {
	broadcaster := ingest.NewSignalBroadcaster()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go broadcaster.Run(ctx)

	mux := chi.NewRouter()
	wsHandler := ingest.NewWebSocketHandler(broadcaster, zap.NewNop())
	wsHandler.RegisterRoutes(mux)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/signals/stream"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	// Publish a signal
	sig := &v1.ArgusSignal{SignalId: "ws-test-001", TraceId: "trace-ws"}
	broadcaster.Publish(sig)

	// Should receive within 1 second
	conn.SetReadDeadline(time.Now().Add(time.Second))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Contains(t, string(msg), "ws-test-001")
}
```

- [ ] **Step 6.3: Run test to see it fail**

```bash
cd C:/Users/Drupad/ArgusXDR && go test ./internal/ingest/... -run TestWebSocket -v 2>&1
```

Expected: FAIL — WebSocketHandler undefined

- [ ] **Step 6.4: Implement WebSocket handler**

Create `internal/ingest/receiver_ws.go`:

```go
package ingest

import (
	"encoding/json"
	"net/http"
	"time"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	// Allow all origins for now (auth will add token check in Step T3)
	CheckOrigin: func(r *http.Request) bool { return true },
}

// WebSocketHandler handles real-time signal streaming over WebSocket.
type WebSocketHandler struct {
	broadcaster *SignalBroadcaster
	log         *zap.Logger
}

// NewWebSocketHandler creates a WebSocket handler backed by the given broadcaster.
func NewWebSocketHandler(b *SignalBroadcaster, log *zap.Logger) *WebSocketHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &WebSocketHandler{broadcaster: b, log: log}
}

// RegisterRoutes mounts the WebSocket endpoint.
func (h *WebSocketHandler) RegisterRoutes(mux *chi.Mux) {
	mux.Get("/v1/signals/stream", h.handleStream)
}

// handleStream upgrades to WebSocket and streams signals until client disconnects.
func (h *WebSocketHandler) handleStream(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Warn("websocket upgrade failed", zap.Error(err))
		return
	}
	defer conn.Close()

	ch := h.broadcaster.Subscribe()
	defer h.broadcaster.Unsubscribe(ch)

	h.log.Debug("websocket client connected",
		zap.String("remote", r.RemoteAddr),
		zap.Int("subscribers", h.broadcaster.SubscriberCount()))

	// Ping/pong keepalive
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	marshaler := protojson.MarshalOptions{EmitUnpopulated: false, UseProtoNames: true}

	for {
		select {
		case sig, ok := <-ch:
			if !ok {
				return // broadcaster closed
			}
			data, err := marshaler.Marshal(sig)
			if err != nil {
				h.log.Warn("failed to marshal signal", zap.Error(err))
				continue
			}
			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				h.log.Debug("websocket write failed (client disconnected)", zap.Error(err))
				return
			}

		case <-pingTicker.C:
			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-r.Context().Done():
			return
		}
	}
}
```

- [ ] **Step 6.5: Wire broadcaster into api.go**

In `cmd/argus/api.go`, update `runAPI` to create the broadcaster and wire it:

```go
// Create signal broadcaster for WebSocket fan-out (P2: non-blocking fan-out)
broadcaster := ingest.NewSignalBroadcaster()
go broadcaster.Run(ctx)

// ... existing query handler setup ...
queryHandler := ingest.NewQueryHandler(ch, httpMetrics, log)
queryHandler.RegisterRoutes(r)

// WebSocket handler
wsHandler := ingest.NewWebSocketHandler(broadcaster, log)
wsHandler.RegisterRoutes(r)
```

- [ ] **Step 6.6: Run tests**

```bash
cd C:/Users/Drupad/ArgusXDR && go test ./internal/ingest/... -run TestWebSocket -v 2>&1
```

Expected: PASS

- [ ] **Step 6.7: Commit**

```bash
git add internal/ingest/broadcaster.go internal/ingest/broadcaster_test.go \
    internal/ingest/receiver_ws.go internal/ingest/receiver_ws_test.go \
    cmd/argus/api.go
git commit -m "feat: WebSocket signal streaming with broadcaster fan-out, per-client buffer"
```

---

## Task 7: Fix Broken Test Compilation

**Files:**
- Modify: `internal/detection/kairos/signal_builder.go` — fix proto field refs
- Modify: `internal/kairos/signal_builder.go` — fix argusv1 import
- Modify: `internal/metrics/redis_monitor_test.go` — fix redis mock types
- Modify: `internal/cmd/doctor.go` — fix net.DialContext

- [ ] **Step 7.1: Fix `internal/detection/kairos/signal_builder.go`**

The file uses old proto field names. Find the broken fields:

```bash
cd C:/Users/Drupad/ArgusXDR && go build ./internal/detection/kairos/... 2>&1
```

For each error like `unknown field TimestampNs`, replace with the current proto field name:
- `TimestampNs` → `Timestamp: timestamppb.New(now)`
- `AppId` → wrap in `Source: &v1.Source{AppId: ...}`
- `Layer_LAYER_DECISION` → use correct Layer enum value from proto
- `Category_CATEGORY_SECURITY` → use correct enum or string

Run `grep -n "Timestamp\|AppId\|Layer_\|Category_" gen/go/argus/v1/*.go` to find current field/enum names, then update signal_builder.go to match.

- [ ] **Step 7.2: Fix `internal/kairos/signal_builder.go`**

```bash
cd C:/Users/Drupad/ArgusXDR && go build ./internal/kairos/... 2>&1 | head -10
```

The file references `argusv1` which isn't imported. Add correct import:
```go
import (
    v1 "github.com/argusxdr/argus/gen/go/argus/v1"
)
```
Then replace all `argusv1.` with `v1.`

- [ ] **Step 7.3: Fix `internal/cmd/doctor.go`**

`net.DialContext` was removed in Go 1.18. Replace with:
```go
// WRONG: net.DialContext(ctx, "tcp", addr)
// RIGHT:
d := &net.Dialer{}
conn, err := d.DialContext(ctx, "tcp", addr)
```

- [ ] **Step 7.4: Fix `internal/metrics/redis_monitor_test.go`**

The test uses `*redis.StringCmd` as a concrete type in mocks. Replace with interface-based mocking or use `redis.NewStringCmd` constructors:
```go
// Instead of: return &mockStringCmd{val: "val"}
// Use redis test helpers or restructure to accept interface
```

The simplest fix: restructure `NewRedisMonitor` to accept an interface instead of `*redis.Client`.

- [ ] **Step 7.5: Run tests to verify compilation fixed**

```bash
cd C:/Users/Drupad/ArgusXDR && go build ./... 2>&1 | grep -v "^#"
```

Expected: No output (all packages compile)

- [ ] **Step 7.6: Run full test suite**

```bash
cd C:/Users/Drupad/ArgusXDR && go test ./... 2>&1 | grep -E "^ok|^FAIL|^---"
```

Expected: All packages compile; some tests may still fail (investigate failures)

- [ ] **Step 7.7: Commit**

```bash
git add internal/ && git commit -m "fix: repair broken test compilation (proto fields, redis mocks, net.Dialer)"
```

---

## Task 8: Enrich Health Endpoint (Step 1.6)

**Files:**
- Modify: `cmd/argus/api.go` — expand `makeHealthHandler`

- [ ] **Step 8.1: Write the test**

```go
func TestHealthHandler_DegradedWhenNilStorage(t *testing.T) {
	// makeHealthHandler is tested via the exported function
	// This tests the response shape
	handler := makeHealthHandler(nil, zap.NewNop())
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, 200, w.Code)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "degraded", body["status"])
	assert.NotNil(t, body["components"])
}
```

- [ ] **Step 8.2: Update `makeHealthHandler` to match build prompt spec**

```go
func makeHealthHandler(ch *storage.ClickHouse, log *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		type componentHealth struct {
			Status    string `json:"status"`
			LatencyMs int64  `json:"latency_ms,omitempty"`
			Error     string `json:"error,omitempty"`
		}

		chComp := componentHealth{Status: "unknown"}
		overall := "healthy"

		if ch == nil {
			chComp = componentHealth{Status: "unhealthy", Error: "not configured"}
			overall = "degraded"
		} else {
			start := time.Now()
			if err := ch.Ping(ctx); err != nil {
				chComp = componentHealth{Status: "unhealthy", Error: err.Error()}
				overall = "degraded"
			} else {
				chComp = componentHealth{Status: "healthy", LatencyMs: time.Since(start).Milliseconds()}
			}
		}

		type healthResponse struct {
			Status     string                     `json:"status"`
			Components map[string]componentHealth `json:"components"`
		}

		resp := healthResponse{
			Status: overall,
			Components: map[string]componentHealth{
				"clickhouse": chComp,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}
}
```

- [ ] **Step 8.3: Run tests and build**

```bash
cd C:/Users/Drupad/ArgusXDR && go build ./cmd/argus && go test ./internal/ingest/... -v 2>&1 | tail -20
```

- [ ] **Step 8.4: Commit**

```bash
git add cmd/argus/api.go
git commit -m "feat: enrich /health with component-level status (ClickHouse latency)"
```

---

## Task 9: Final Verification

- [ ] **Step 9.1: Start infra and server**

```bash
# Start Docker services (requires Docker Desktop running)
docker compose up -d clickhouse postgres redis

# Wait for healthy
docker compose ps

# Start API server
go run ./cmd/argus api --dev
```

- [ ] **Step 9.2: Hit every route, verify non-404**

```bash
# Tier 1
curl -s http://localhost:8080/health
curl -s http://localhost:8080/v1/signals?app_id=test
curl -s http://localhost:8080/v1/schema/signals | python3 -m json.tool | head -10
curl -s http://localhost:8080/api/v1/layers/status | python3 -m json.tool
curl -s http://localhost:8080/api/v1/traces/test-trace
curl -s -X POST -H "Content-Type: application/json" \
    -d '{"sql":"SELECT count() FROM signals"}' \
    http://localhost:8080/api/v1/query

# Tier 2
curl -s http://localhost:8080/api/v1/rules
curl -s http://localhost:8080/api/v1/alerts
curl -s http://localhost:8080/api/v1/incidents

# Tier 3
curl -s -X POST http://localhost:8080/api/v1/auth/login
curl -s http://localhost:8080/api/v1/users
curl -s http://localhost:8080/api/v1/apps
```

Expected: All return 200, 400, 501, or 503 — NEVER 404.

- [ ] **Step 9.3: Run full test suite**

```bash
cd C:/Users/Drupad/ArgusXDR && go test ./... 2>&1 | grep -E "^ok|^FAIL"
```

Expected: No build failures. Any test failures are known and tracked.

- [ ] **Step 9.4: Final commit**

```bash
git add -A
git commit -m "feat: Step 1 complete — all routes non-404, WebSocket streaming, graceful degradation"
```

---

## Self-Review Against Spec

| Spec Requirement | Task | Status |
|-----------------|------|--------|
| Fix chi router (Step 1.1) | Task 0 pre-work | ✓ Fixed (log-before-bind) |
| Route registration test (Step 1.2) | Task 1 | ✓ |
| Golden signal metrics on handlers (Step 1.3) | TODO — add Prometheus middleware in api.go | ⚠ Partial |
| GET /api/v1/layers/status (Step 1.4 T1) | Task 0 pre-work | ✓ |
| GET /v1/signals/stream WebSocket (Step 1.4 T1) | Task 6 | ✓ |
| GET /api/v1/schema/signals (Step 1.4 T1) | Task 0 pre-work | ✓ |
| GET /api/v1/traces/{traceId} (Step 1.4 T1) | Task 3 | ✓ |
| POST /api/v1/query (Step 1.4 T1) | Task 4 | ✓ |
| CRUD /api/v1/rules (Step 1.4 T2) | Task 1 stubs → Step 2 plan | 501 stubs |
| CRUD /api/v1/alerts (Step 1.4 T2) | Task 1 stubs → Step 2 plan | 501 stubs |
| CRUD /api/v1/incidents (Step 1.4 T2) | Task 1 stubs → Step 2 plan | 501 stubs |
| Auth endpoints (Step 1.4 T3) | Task 1 stubs → Step 4 plan | 501 stubs |
| SignalBroadcaster (Step 1.5) | Task 5 | ✓ |
| Health endpoint with components (Step 1.6) | Task 8 | ✓ |
| Golden signal metrics on all handlers | Not yet in plan | ❌ Missing |

**Gap identified:** Golden signal metrics (Prometheus histogram middleware) not yet in plan. Add to Task 8 or as Task 10.
