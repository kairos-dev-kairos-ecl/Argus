# Argus XDR v1.0 — Master Roadmap

> Derived from ARGUS_FINAL_BUILD_PROMPT.md. Each step has its own detailed plan.

## Overall Progress

| Step | Plan File | Status | Gate |
|------|-----------|--------|------|
| Step 0 | `.planning/AUDIT.md` | ✅ Complete | Audit doc complete |
| Step 1 | `2026-04-12-argus-xdr-v1-step1-data-fabric.md` | 🔄 In Progress | Every route non-404 |
| Step 2 | `2026-04-12-argus-xdr-v1-step2-detection.md` | ⏳ Pending | All 15 rules load, Tier 1-3 pass tests |
| Step 3 | `2026-04-12-argus-xdr-v1-step3-alert-routing.md` | ⏳ Pending | Detection→alert→notify E2E |
| Step 4 | `2026-04-12-argus-xdr-v1-step4-auth.md` | ⏳ Pending | Login→token→RBAC→audit |
| Step 5 | `2026-04-12-argus-xdr-v1-step5-frontend.md` | ⏳ Pending | Full user journey works |
| Step 6 | `2026-04-12-argus-xdr-v1-step6-packaging.md` | ⏳ Pending | `curl install.sh \| sh` works |
| Step 7 | `2026-04-12-argus-xdr-v1-step7-test-harness.md` | ⏳ Pending | ≥85% detection rate |
| Step 8-10 | `2026-04-12-argus-xdr-v1-step8-validation.md` | ⏳ Pending | All checks pass |

---

## What's Already Working (Do Not Rewrite)

- ✅ Protobuf schemas (ArgusSignal, L1-L10 typed contexts)
- ✅ gRPC + OTLP ingest receivers
- ✅ 7-stage processing pipeline (validate→normalize→correlate→enrich→baseline→route)
- ✅ Fixed goroutine pool (GOMAXPROCS×2), async baseline (10-min cycle)
- ✅ ClickHouse storage (ReplacingMergeTree, 80 columns, monthly partitions)
- ✅ PostgreSQL (users, sessions, audit_log — from migration 007_auth)
- ✅ Redis (correlation, dedup, rate limiting)
- ✅ Notify adapters (Slack, PagerDuty, Webhook, Email, Syslog) — **tests passing**
- ✅ Chi routing bug fixed (log-before-bind → bind-first pattern)
- ✅ API graceful degradation (starts without ClickHouse, returns 503 per endpoint)

---

## What Step 1 Delivers

- All API routes registered and non-404
- WebSocket signal streaming  
- `/api/v1/layers/status` — layer health for dashboard coverage map
- `/api/v1/traces/{traceId}` — trace detail
- `POST /api/v1/query` — safe SQL with DDL blocking
- Tier 2/3 routes as 501 stubs (alerts, incidents, rules, auth, users, apps)
- Component-level `/health` endpoint
- Broken test compilation fixed (14 packages)
- PostgreSQL tables for apps, rules, alerts, incidents, channels

---

## Step 2 Preview: Detection Engine

**Current state:** Kairos remote HTTP evaluator works but signal_builder.go has proto mismatches (breaks build). No local YAML rules.

**Build prompt expects:** Tier 1 (deterministic field comparison), Tier 2 (statistical/baseline), Tier 3 (temporal correlation patterns), 15 built-in YAML rules.

**Plan will cover:**
- Fix signal_builder.go proto mismatches
- Write local YAML rule evaluator (Tier 1: field comparison, no external dependency)
- Wire baseline scorer (Tier 2: z-score from enrichment.baseline_deviation)
- Implement temporal pattern matching via Redis sorted sets (Tier 3)
- Wire rule management API (CRUD, hot-reload without restart)
- Seed 15 built-in rules covering MITRE ATLAS threats

---

## Step 3 Preview: Alert Routing

**Current state:** `internal/notify` adapters exist and tests pass. No alert router, no alert table writes, no dedup.

**Plan will cover:**
- Alert router: detection → fingerprint → dedup check → PostgreSQL write → route to channel
- Priority-based delivery (critical first when adapters overloaded)
- Circuit breaker per adapter (3-failure→open, 30s half-open)
- Alert dedup (15-min fingerprint window)
- Incident auto-correlation (≥3 alerts, same trace_id or app_id within 10 min)
- Wire CRUD handlers for `/api/v1/alerts` and `/api/v1/incidents`

---

## Step 4 Preview: Auth System

**Current state:** Auth package exists but doesn't compile. Has JWT, RBAC, sessions, audit table. 

**Plan will cover:**
- Fix auth package compilation
- Wire all auth endpoints (login, refresh, logout, setup, change-password)
- Account lockout (5 failures → 15-min lock)
- Refresh token rotation (invalidate old on each use)
- RBAC middleware (ProtectedRoute handler wrapper for admin/analyst/viewer)
- `POST /api/v1/auth/setup` — only works when 0 users exist (first-run)

---

## Architectural Principles Applied Everywhere

Per ARGUS_FINAL_BUILD_PROMPT.md — these are requirements, not aspirations:

| Principle | Implementation |
|-----------|---------------|
| P1: Four Golden Signals | Prometheus middleware on every handler |
| P2: Explicit Backpressure | Bounded channels, 429/RESOURCE_EXHAUSTED, load shedding |
| P3: Graceful Degradation | Every component failure returns degraded state, not crash |
| P4: span_kind enum | Add to proto schema + ClickHouse table in Step 2 |
| P5: Structured annotations | metrics/metadata/tags maps in proto + ClickHouse in Step 2 |
| P6: Circuit Breakers | Per adapter in alert routing (Step 3) |
