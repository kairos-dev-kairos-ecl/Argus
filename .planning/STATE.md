---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
current_plan: 1
status: executing
last_updated: "2026-04-24T14:29:22.916Z"
progress:
  total_phases: 6
  completed_phases: 3
  total_plans: 21
  completed_plans: 23
---

# ArgusXDR — Project State

> Last activity: 2026-04-24 - Completed Phase 6 Plan 6: TOTP Primitives

---

## Current Status

**Active phase:** Phase 6 — Security Hardening: Zero-Trust Auth & API Protection (executing)
**Current plan:** 1
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
- `internal/auth/` — JWT (RS256), API keys, RBAC, TOTP primitives + backup codes
- `internal/detection/kairos/` — Kairos HTTP policy evaluator (build failures)
- `migrations/007_auth.up.sql` — users, sessions, audit_log, token_revocations
- `migrations/010_mfa.up.sql` — mfa_enabled, mfa_secret_encrypted, user_backup_codes table

### Proto Schema

- `proto/argus/v1/signal.proto` — ArgusSignal (L1–L10 + LDecision)
- All 11 context layers have real field definitions (completed in 260416-lxk)
- `gen/go/argus/v1/` — Generated Go stubs (from current proto)
- Round-trip validated via `tests/unit/signal/signal_layers_test.go` (all 11 pass)

### SDK

- `sdk/client.py` — Python ArgusClient (httpx, async)
- `sdk/signal_builder.py` — SignalBuilder (fluent API)
- `sdk/typescript/src/` — TypeScript client + builder
- `test_harness/` — qwen_llama_api.py + validate_signals.py

### Frontend (React + TypeScript)

- `web/src/` — 22 pages, 20+ components
- Zustand stores: auth, signal filters, trace view
- TanStack Query hooks, WebSocket listener
- **Status:** Executing Phase 06

---

## Known Issues

| Issue | Severity | Notes |
|-------|----------|-------|
| 14 packages won't compile | RESOLVED | Fixed in quick task 260416-m9t — all packages build clean |
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
- **Auth context storage:** Single canonical claimsKey constant for context.Context value storage (backward compatible with old ContextKeyUser).
- **HIBP fail-open:** Network errors during password breach check allow setup to proceed (security-first, usability-second).
- **RBAC granularity:** Permission-based (not role-based) for read endpoints to allow admin/analyst to carry same permissions as viewer role.

---

## Blockers/Concerns

- Go stub regeneration requires `buf` CLI installed
- ClickHouse DDL has drifted from proto schema (needs sync)

---

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 1 | Proto schema rewrite — all 10 layers + LDecision (14 reqs) | 2026-04-16 | f1606bc | 260416-lxk-proto-schema-rewrite-covering-all-10-lay |
| 2 | Build stabilization + signal validation tests (all 11 layers) | 2026-04-16 | 8edbd43 | 260416-m9t-build-stabilization-and-signal-validatio |
| 3 | Backend validation harness: docker-compose-test.yml + llama.cpp + signal capture | 2026-04-19 | 16adae2 | 260419-tmi-set-up-backend-validation-harness-docker |
| 4 | Commit session changes, clean up stale files, update Obsidian docs to reflect current architecture | 2026-04-22 | ac0f054 | 260423-0c1-commit-session-changes-clean-up-stale-fi |

### Plans Completed

| Phase | Plan | Name | Date | Commits | Status |
|-------|------|------|------|---------|--------|
| 6 | 1 | Security Hardening — RBAC Middleware & HIBP | 2026-04-24 | 97d6206, b40eb83, 6e54a17 | Complete |
| 6 | 2 | Rate Limiting — Wave 2 | 2026-04-24 | af26f4f, b055b90 | Complete |
| 6 | 3 | API Key Schema & CRUD | 2026-04-24 | 5c364cb, b26674f, b84178a | Complete |
| 6 | 4 | Secrets File Architecture — Wave 3 | 2026-04-24 | 9b3634f, e1cd8b6, 2923a22, 8565a50 | Complete |
| 6 | 6 | TOTP Primitives — Wave 4 | 2026-04-24 | 9078ef5, 5776217, d677532 | Complete |
