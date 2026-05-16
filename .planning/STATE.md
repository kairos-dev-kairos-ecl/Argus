---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
current_plan: 2
status: executing
last_updated: "2026-05-16T14:40:00.000Z"
progress:
  total_phases: 7
  completed_phases: 1
  total_plans: 19
  completed_plans: 27
---

# ArgusXDR — Project State

> Last activity: 2026-05-16 - Completed Phase 7 Plan 01: Wave 0 data foundation — ClickHouse bloom-filter skip indexes on session_id/conversation_id + PostgreSQL migration 011 session_baseline_profiles

---

## Current Status

**Active phase:** Phase 7 — Behavioural Traceability & TUI (executing)
**Current plan:** 2
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
- **Status:** Executing Phase 07

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
- **Tailwind v4 CSS entrypoint:** globals.css (not index.css) is the actual CSS entrypoint imported by main.tsx; both files updated for brutalist theme.
- **Tailwind v4 theme config:** tailwind.config.js CSS var references used for color tokens — allows live token editing without rebuild.
- **Phase 7 skip indexes:** bloom_filter(0.01) GRANULARITY 4 for session_id/conversation_id — 1% FP rate for high-cardinality string IDs; non-fatal on failure so server still starts.
- **session_baseline_profiles expires_at NOT NULL:** Forces callers (SessionBaselineEngine) to supply explicit expiry; prevents accidental indefinite retention.

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
| 5 | Phase 5 manual validation fixes: Sankey, custom date range, trace discovery, Audit/Sessions/Incidents/API Keys 500s, IAM matrix | 2026-04-27 | 8351126 | (inline) |
| 6 | Browser console error fixes: audit DTO snake_case, trace query column names, signals datetime filter typo (parseDatetime64→parseDateTime64), settings 401 graceful fallback | 2026-04-28 | bfaf98a, bf74189 | (inline) |
| 7 | User onboarding first-run setup wizard + invite flow: setup-status endpoint, auto-login after setup, invite migration+service+handlers, rewritten SetupWizard, AcceptInvitePage, UsersPage invite button | 2026-04-30 | 20a44ab, 43fc5e6 | 260430-nxf-implement-user-onboarding-first-run-setu |
| 8 | Backdrop image + clear navigation flow: Login shows 'FIRST TIME SETUP →' and 'HAVE AN INVITE →' links; SetupWizard has '← BACK TO LOGIN'; AcceptInvitePage has '← LOGIN' header link; backdrop.png on all auth pages | 2026-05-01 | 1ae83ff | 260501-h8b-add-backdrop-image-to-login-setup-pages- |
| 9 | One-click E2E validation script (argus-e2e-validate.sh): auto-bootstrap admin + API key, 7-scenario L1-L10 signal injection (70 signals), trace validation, JSON report, --skip-llm fallback | 2026-05-01 | 5f60b9c | (inline) |
| 10 | Interface selector + TUI implementation plan: CONTEXT.md (4 decisions locked) + 4-phase PLAN.md covering CLI selector, Bubbletea TUI foundation, 6 core operator screens, UX polish (~29 net-new Go files, 0 backend changes) | 2026-05-08 | — | 260508-mis-interface-selector-and-tui-implementatio |
| 11 | TUI Phase 1 — Interface selector + Cobra restructure: selector Bubbletea app, prefs layer (atomic write, 0600), argus web/tui/reset-ui subcommands, root dispatch (pref-check->selector->save->dispatch) | 2026-05-08 | 7dc02f6, fe650ac, c238b81 | 260508-mt2-tui-phase-1-interface-selector-and-cobra |
| 12 | TUI Phase 2 — Bubbletea root model + Lipgloss theme: brutalist theme constants, JWT-in-memory AuthState, HTTP APIClient (401 refresh-retry), WSClient (Authorization header on Upgrade), login screen with MFA branch, 6 placeholder screens, help overlay, quit confirm | 2026-05-08 | da8cd2b, 49f1c97, 463c48e | 260508-n7t-tui-phase-2-bubbletea-root-model-lipglos |
| 13 | TUI Phase 3 — 6 live operator screens: signals (WS stream + filter), trace (expand/collapse tree), alerts (ack/resolve confirm), rules ($EDITOR secure temp file), users (invite + deactivate confirm, admin-only), audit (viewport + filter, admin-only); security grep all PASS | 2026-05-08 | e90f4fe, 007f373, c85f434 | 260508-te7-tui-phase-3-6-live-operator-screens-live |
| 14 | TUI Phase 4 — UX polish: sectioned ? help overlay (keys/bindings.go single source of truth), --no-unicode SSH/dumb-terminal fallback + auto-detection, q quit confirm polish ("Quit Argus TUI? [y/N]"), keymap consolidation + deviation docs across 6 screens, argus tui --version | 2026-05-08 | 5451341, f75d2c3 | 260508-tza-tui-phase-4-ux-polish-help-overlay-no-un |
| 15 | Fix frontend WebSocket live-test issues: remove app_id='test' filter blocking non-test signals, suppress StrictMode CONNECTING-abort console noise via handler-detach-before-close, add pendingReject to WebSocketClient to prevent promise leak | 2026-05-09 | e971850, 3f87ae6, 04a52d8 | 260509-vvd-fix-frontend-websocket-live-test-issues |
| 16 | Fix ClickHouse timestamp corruption: pass time.Time (not int64) for DateTime64 timestamp + ingested_at columns — clickhouse-go/v2 unconditionally treats int64 as ms so UnixNano caused year 2228+ garbage timestamps | 2026-05-09 | ae0fb29 | 260509-wcz-fix-clickhouse-timestamp-corruption-pass |

### Plans Completed

| Phase | Plan | Name | Date | Commits | Status |
|-------|------|------|------|---------|--------|
| 6 | 1 | Security Hardening — RBAC Middleware & HIBP | 2026-04-24 | 97d6206, b40eb83, 6e54a17 | Complete |
| 6 | 2 | Rate Limiting — Wave 2 | 2026-04-24 | af26f4f, b055b90 | Complete |
| 6 | 3 | API Key Schema & CRUD | 2026-04-24 | 5c364cb, b26674f, b84178a | Complete |
| 6 | 4 | Secrets File Architecture — Wave 3 | 2026-04-24 | 9b3634f, e1cd8b6, 2923a22, 8565a50 | Complete |
| 6 | 6 | TOTP Primitives — Wave 4 | 2026-04-24 | 9078ef5, 5776217, d677532 | Complete |
| 6 | 7 | TOTP Handlers & Login Branching — Wave 5 | 2026-04-24 | 1e7a7e9, bd477c5, a74c6fa | Complete |
| 6 | 8 | Session Management & CSRF Protection — Wave 6 (FINAL) | 2026-04-24 | 71eb040, 752d3b7, 608f977 | Complete |
| 5 | 1 | Design System Tokens Reset (brutalist theme) | 2026-04-26 | a558f44, 673ca7a, 2cb0fff | Complete |
| 5 | 8 | Incidents MITRE ATLAS Screen (Screen 6) | 2026-04-27 | 1aa51cd, 2db4f8a | Complete |
| 7 | 1 | Wave 0 Data Foundation — ClickHouse Skip Indexes + PG Migration 011 | 2026-05-16 | 87289ea, 54830f7 | Complete |
