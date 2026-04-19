# Module Dependencies & Cross-References

> Complete interconnection map. Who calls whom, and when.

## Signal Flow (Root Dependency)

```
[[Signal — ArgusSignal Schema]]  ← The contract — all other modules depend on this
  ├── [[SDK]]  writes signals
  ├── [[Ingest Pipeline]]  receives + processes
  ├── [[Storage Layer]]  persists
  ├── [[Detection Engine]]  matches against
  ├── [[Notify & Alerting]]  routes + dispatches
  ├── [[Web Dashboard]]  displays
  └── [[Baseline Engine]]  profiles
```

---

## Detailed Dependency Graph

### Entry Points

#### SDK → Ingest Pipeline
```
[[SDK#ArgusClient.emit_signal()]] 
  POST /v1/signals
  ↓
[[Ingest Pipeline#HTTPReceiver]]
  ├─ [[Auth & RBAC#API Key Auth]]  (PostgreSQL api_keys lookup)
  └─ queue.Enqueue()
```

#### Ingest Pipeline Chain

```
[[Ingest Pipeline#WorkerPool]]
  → pull from ingest.Queue
  → [[Ingest Pipeline#SchemaValidator]]
  → [[Ingest Pipeline#Normalizer]]
  → [[Ingest Pipeline#CorrelationTagger]]
       ├─ [[Storage Layer#Redis]]  trace:* keys
       └─ populate signal.related_signals[]
  → [[Ingest Pipeline#BaselineScorer]]
       ├─ [[Baseline Engine#ProfileStore]]
       │  ├─ [[Storage Layer#Redis]]  baseline:* cache
       │  └─ [[Storage Layer#PostgreSQL]]  baseline_profiles
       └─ compute z-score → signal.enrichment.baseline_deviation
  → [[Ingest Pipeline#Enricher]]
       └─ [[Ingest Pipeline#GeoIP enrichment]]
            ├─ MaxMind geoip2 database (disk)
            └─ [[Storage Layer#Redis]]  geoip:* cache (24h TTL)
  → [[Ingest Pipeline#DetectionProcessor]]
       └─ [[Detection Engine#DetectionEngine.Evaluate()]]
            ├─ [[Detection Engine#Tier 1]]  (static matching)
            ├─ [[Detection Engine#Tier 2]]  (baseline deviation)
            │  └─ uses enrichment.baseline_deviation (already set)
            ├─ [[Detection Engine#Tier 3]]  (temporal frequency)
            │  └─ [[Storage Layer#Redis]]  det:t3:* sorted sets
            ├─ → (optional) [[Detection Engine#Kairos]]
            │  └─ HTTP call to external policy engine
            └─ on match → [[Notify & Alerting#AlertRouter]]
  → [[Storage Layer#ClickHouse]]  BatchWriter
       └─ signals table (batch 500, flush 2s)
  → [[Ingest Pipeline#SignalBroadcaster]]
       └─ WebSocket /v1/signals/stream
```

### Alerting Pipeline

```
[[Detection Engine]]  match
  → [[Notify & Alerting#AlertRouter]]
       ├─ Redis dedup check: dedup:{fingerprint} SET NX 10m
       ├─ [[Storage Layer#PostgreSQL]]  alerts table upsert
       ├─ Incident correlation: ≥3 alerts, same trace, 10 min → incidents table
       └─ [[Notify & Alerting#RoutingEngine]]
            ├─ Load routing_rules from [[Storage Layer#PostgreSQL]] (5 min hot-reload)
            ├─ Check [[Notify & Alerting#SuppressionEngine]]
            │  └─ suppression_rules in PostgreSQL
            └─ → [[Notify & Alerting#AlertDispatcher]]
                 └─ 4-worker pool
                    → Notifier adapter (Slack / PagerDuty / Email / Webhook / Syslog)
```

### Background Tasks

#### Baseline Engine

```
[[Baseline Engine#BaselineEngine]]  (every 10 min)
  → Query [[Storage Layer#ClickHouse]]  signals table
       WHERE timestamp > now() - 24h
       GROUP BY (app_id, layer, category)
       HAVING count(*) >= 100
  → [[Baseline Engine#ProfileCalculator]]
       → mean, stddev, min, max
  → [[Baseline Engine#ProfileStore]]
       ├─ [[Storage Layer#Redis]]  baseline:{app}:{layer}:{cat}  (5 min cache)
       └─ [[Storage Layer#PostgreSQL]]  baseline_profiles  (durable)
```

#### GeoIP Updater

```
[[Ingest Pipeline#GeoIPEnricher]]  (daily refresh)
  → Check MaxMind DB age
  → If stale: download latest from MaxMind API
  → Load into geoip2 reader
  → Clear [[Storage Layer#Redis]]  geoip:* cache
```

---

## HTTP Endpoints Cross-Reference

### Signal Ingest
```
[[API Routes#Ingest Endpoints]]
POST /v1/signals
  ← [[SDK#ArgusClient.emit_signal()]]
  → [[Ingest Pipeline#HTTPReceiver]]
  → [[Auth & RBAC#API Key Auth]]
  → ingest.Queue → [[Ingest Pipeline#WorkerPool]]
```

### Signal Query
```
[[API Routes#Query Endpoints]]
GET /v1/signals
  ← [[Web Dashboard]]  (TanStack Query hook)
  → [[Ingest Pipeline#receiver_query.go]]
  → [[Storage Layer#ClickHouse]]  keyset pagination
  
GET /api/v1/traces/{traceId}
  ← [[Web Dashboard#TracePage]]
  → [[Ingest Pipeline#receiver_query.go]]
  → [[Storage Layer#ClickHouse]]  WHERE trace_id = ?
```

### Rules Management
```
[[API Routes#Rules]]
GET /api/v1/rules
  ← [[Web Dashboard#RulesPage]]
  → [[Detection Engine#Loader]]
  → [[Storage Layer#PostgreSQL]]  detection_rules table

POST /api/v1/rules/test
  ← [[Web Dashboard]]  (analyst testing)
  → [[Detection Engine#DetectionEngine.Evaluate()]]
  → against provided signal
```

### Alerts
```
[[API Routes#Alerts]]
GET /api/v1/alerts
  ← [[Web Dashboard#AlertsPage]]
  → [[Storage Layer#PostgreSQL]]  alerts table

POST /api/v1/alerts/{id}/acknowledge
  ← [[Web Dashboard]]  (analyst action)
  → [[Notify & Alerting#AlertRouter]]
  → UPDATE alerts.status = 'acknowledged'
```

### Auth
```
[[API Routes#Auth]]
POST /api/v1/auth/login
  ← [[Web Dashboard#LoginPage]]
  → [[Auth & RBAC#JWT Auth]]
  → Issue RS256 token (15 min TTL)
  → Set refresh cookie

POST /api/v1/auth/refresh
  ← Silent refresh ([[Web Dashboard]])
  → Validate refresh cookie
  → Check [[Storage Layer#PostgreSQL]]  sessions table (not revoked)
  → Issue new access token
```

---

## Storage Connections

### ClickHouse

```
[[Storage Layer#ClickHouse]]
  ← [[Ingest Pipeline#BatchWriter]]  (signals)
  ← [[Baseline Engine]]  (query for profiles)
  ← [[API Routes#Query Endpoints]]  (GET /v1/signals, /api/v1/traces/*, /api/v1/layers/status)
  ← [[Web Dashboard#QueryPage]]  (raw SQL POST /api/v1/query)
  ← [[Web Dashboard#DashboardPage]]  (layer status)
```

### PostgreSQL

```
[[Storage Layer#PostgreSQL]]
  ← [[Auth & RBAC#UserService]]  (users, sessions, roles)
  ← [[Ingest Pipeline#AuthValidator]]  (api_keys lookup)
  ← [[Notify & Alerting#AlertRouter]]  (alerts, incidents upsert)
  ← [[Notify & Alerting#RoutingEngine]]  (routing_rules query)
  ← [[Notify & Alerting#SuppressionEngine]]  (suppression_rules query)
  ← [[Baseline Engine#ProfileStore]]  (baseline_profiles persist)
  ← [[Detection Engine#Loader]]  (detection_rules query)
  ← [[API Routes#Rules]]  (POST /api/v1/rules CRUD)
  ← [[Web Dashboard#UsersPage]]  (user CRUD)
```

### Redis

```
[[Storage Layer#Redis]]
  ← [[Ingest Pipeline#CorrelationTagger]]  (trace:* ZSET)
  ← [[Ingest Pipeline#BaselineScorer]]  (baseline:* GET cache)
  ← [[Ingest Pipeline#Enricher#GeoIP]]  (geoip:* cache)
  ← [[Ingest Pipeline#DetectionProcessor#Tier3]]  (det:t3:* temporal count)
  ← [[Notify & Alerting#AlertRouter]]  (dedup:{fp} SET NX)
  ← [[Auth & RBAC#AuthValidator]]  (api_keys cache)
```

---

## Configuration Connections

```
[[Config & Environment]]
  ← [[Deployment#Makefile]]  (make run-api sets ARGUS_* env vars)
  ← [[Deployment#docker-compose.yml]]  (services env blocks)
  ← Command-line flags (viper.BindPFlag)
  → Used by:
    ├─ [[Ingest Pipeline#WorkerPool]]  (GOMAXPROCS × 2)
    ├─ [[Storage Layer#ClickHouse]]  (CLICKHOUSE_DSN)
    ├─ [[Storage Layer#PostgreSQL]]  (POSTGRES_DSN)
    ├─ [[Storage Layer#Redis]]  (REDIS_ADDR)
    ├─ [[Baseline Engine]]  (history_window, compute_interval)
    └─ [[Detection Engine#Loader]]  (rules_dir)
```

---

## Dashboard-Backend Connections

```
[[Web Dashboard]]
  ├─ TanStack Query hooks
  │  ├─ useSignals()  → [[API Routes#Query Endpoints]] GET /v1/signals
  │  ├─ useAlerts()   → GET /api/v1/alerts
  │  ├─ useRules()    → GET /api/v1/rules
  │  └─ useTrace()    → GET /api/v1/traces/{id}
  │
  ├─ WebSocket listener
  │  └─ [[Ingest Pipeline#SignalBroadcaster]]  /v1/signals/stream
  │
  ├─ Mutations (TanStack Query)
  │  ├─ useMutateAcknowledgeAlert()  → POST /api/v1/alerts/{id}/acknowledge
  │  ├─ useMutateCreateRule()        → POST /api/v1/rules
  │  └─ useMutateLogin()             → POST /api/v1/auth/login
  │
  └─ Zustand stores
     ├─ auth.ts  ← [[Auth & RBAC#JWT Auth]]
     ├─ signal.ts  ← filter state
     └─ traceViewStore.ts  ← trace detail state
```

---

## Testing & Validation

```
[[SDK#Test Harness]]
  ├─ qwen_llama_api.py
  │  ├─ LlamaCppClient  (external llama.cpp server)
  │  ├─ [[SDK#ArgusClient]]
  │  │  → [[Ingest Pipeline#HTTPReceiver]]  POST /v1/signals
  │  │  → [[Ingest Pipeline#WorkerPool]]
  │  │  → [[Storage Layer#ClickHouse]]
  │  └─ Emits all 10 layers (L1–L10)
  │
  └─ validate_signals.py
     ├─ Query [[API Routes#Query Endpoints]]  GET /v1/signals
     ├─ Verify [[Storage Layer#ClickHouse]]  signal count per layer
     ├─ Check [[Signal — ArgusSignal Schema]]  schema compliance
     └─ Validate [[Ingest Pipeline#CorrelationTagger]]  trace correlation
```

---

## Failure Modes & Recovery

### ClickHouse Down
```
[[Storage Layer#ClickHouse]]  unavailable
  → [[Ingest Pipeline#BatchWriter]]  fails
  → [[Ingest Pipeline#WorkerPool]]  backpressure
  → ingest.Queue  fills (100K cap)
  → [[Ingest Pipeline#HTTPReceiver]]  returns 429
  → [[SDK#ArgusClient]]  receives backpressure
  
Recovery:
  1. ClickHouse comes up
  2. BatchWriter retry-once succeeds
  3. Queue drains
  4. Ingest resumes
```

### PostgreSQL Down
```
[[Storage Layer#PostgreSQL]]  unavailable
  → [[Auth & RBAC#API Key Auth]]  fails on first signal
  → All subsequent signals fail auth (no PG cache hit)
  → [[Ingest Pipeline#HTTPReceiver]]  returns 401
  
OR
  → [[Notify & Alerting#AlertRouter]]  can't upsert alerts
  → [[Notify & Alerting#RoutingEngine]]  can't load routing rules
  → Alerts not dispatched to Slack/email/etc.
  
Recovery:
  1. PostgreSQL comes up
  2. Auth validator rebuilds cache
  3. AlertRouter retries buffered alerts
```

### Redis Down
```
[[Storage Layer#Redis]]  unavailable
  → [[Ingest Pipeline#CorrelationTagger]]  skipped (no trace correlation)
  → [[Ingest Pipeline#BaselineScorer]]  falls back to PG (slower)
  → [[Detect ion Engine#Tier3]]  disabled (no temporal frequency)
  → Dedup disabled (no 15-min window)
  
→ API still starts, signals still ingest, some features degrade
```

---

## File Dependency Map

```
cmd/argus/
  main.go       → viper + cobra init
  api.go        → all service init + handler registration
  ├─ storage.ClickHouse          (internal/storage/clickhouse.go)
  ├─ storage.Postgres            (internal/storage/postgres.go)
  ├─ auth.TokenManager           (internal/auth/jwt.go)
  ├─ ingest.Queue                (internal/ingest/queue.go)
  ├─ pipeline.WorkerPool         (internal/pipeline/workers.go)
  ├─ detection.DetectionEngine   (internal/detection/engine/engine.go)
  ├─ notify.AlertDispatcher      (internal/notify/dispatcher.go)
  └─ baseline.BaselineEngine     (internal/baseline/engine.go)

internal/
  ingest/receiver_*.go           → all register via receiver_query.go
  pipeline/workers.go            → pulls Chain
  detection/engine/              → loads [[internal/rules/built-in/]]
  storage/migrations/            → auto-applied on Postgres init
```
