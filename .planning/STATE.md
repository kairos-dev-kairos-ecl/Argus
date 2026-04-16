# ArgusXDR — Project State

> Last activity: 2026-04-16 - Completed quick task 260416-lxk: proto schema rewrite (all 10 layers + LDecision)

---

## Current Status

**Active phase:** Phase 1 — Proto Schema Rewrite (planned)
**Milestone:** M1 — Foundation & Observability

---

## What's Been Built

### Infrastructure
- Docker Compose stack: ClickHouse 24, PostgreSQL 16, Redis 7.2
- Multi-stage Dockerfile (builder + runtime)
- Makefile with build, test, lint, docker, proto targets

### Backend (Go)
- `cmd/argus/` — Cobra CLI (api, ingest, server, rules, users, doctor commands)
- `internal/ingest/` — HTTP/gRPC/OTLP receivers, queue (100K cap), batch writer
- `internal/pipeline/` — 7-stage chain (SchemaValidator → Normalizer → CorrelationTagger → BaselineScorer → Enricher → DetectionProcessor → BatchWriter)
- `internal/storage/` — ClickHouse + PostgreSQL clients, schema.go (80+ columns)
- `internal/baseline/` — Async 10-min computation cycle, ProfileStore (Redis + PG)
- `internal/notify/` — Dispatcher + adapters (Slack, PagerDuty, Email, Webhook, Syslog)
- `internal/auth/` — JWT (RS256), API keys, RBAC (build failures present)
- `internal/detection/kairos/` — Kairos HTTP policy evaluator (build failures)
- `migrations/007_auth.up.sql` — users, sessions, audit_log, token_revocations

### Proto Schema
- `proto/argus/v1/signal.proto` — ArgusSignal (L1–L10 + LDecision)
- **Known issue:** L1, L2, L3, L4, L6, L9, L10 are placeholder stubs (`string placeholder = 1`)
- Only L5, L7, L8 have real field definitions
- `gen/go/argus/v1/` — Generated Go stubs (from current proto)

### SDK
- `sdk/client.py` — Python ArgusClient (httpx, async)
- `sdk/signal_builder.py` — SignalBuilder (fluent API)
- `sdk/typescript/src/` — TypeScript client + builder
- `test_harness/` — qwen_llama_api.py + validate_signals.py

### Frontend (React + TypeScript)
- `web/src/` — 22 pages, 20+ components
- Zustand stores: auth, signal filters, trace view
- TanStack Query hooks, WebSocket listener
- **Status:** Architecturally complete; blocked by missing backend endpoints

---

## Known Issues

| Issue | Severity | Notes |
|-------|----------|-------|
| 14 packages won't compile | Critical | Proto field mismatches, missing client package, Redis mock types |
| Proto L1/L2/L3/L4/L6/L9/L10 are placeholders | RESOLVED | Completed in quick task 260416-lxk |
| ClickHouse DDL drifted from proto | High | Has columns with no corresponding proto fields |
| Missing API endpoints (10+) | High | Frontend blocked |
| WebSocket stream not implemented | High | Real-time feed non-functional |
| Missing PostgreSQL tables | High | apps, detection_rules, alerts, incidents, routing_rules |

---

## Key Decisions

- **Observability-first:** ArgusSignal = pure observations. MITRE/OWASP live in Alerts, not signals.
- **Schema-first:** proto is the contract. Everything derives from it.
- **Inference-engine-agnostic:** optional fields per engine, no engine-specific structs.
- **Kairos:** Sidecar now → capability differentiator later (Approach C).
- **Tech stack locked:** Go core, ClickHouse signals, PostgreSQL config, Redis ephemeral, React+TS.

---

## Blockers/Concerns

- Proto rewrite must happen before build stabilization (Phase 2 depends on Phase 1)
- Go stub regeneration requires `buf` CLI installed

---

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 1 | Proto schema rewrite — all 10 layers + LDecision (14 reqs) | 2026-04-16 | f1606bc | 260416-lxk-proto-schema-rewrite-covering-all-10-lay |
