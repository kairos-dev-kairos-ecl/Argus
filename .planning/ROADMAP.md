# ArgusXDR — Roadmap

> Bootstrapped: 2026-04-16 | From AUDIT.md codebase state

---

## Milestone: M1 — Foundation & Observability

**Goal:** Production-grade XDR platform for LLM systems with full-spectrum observability across the 10-layer taxonomy.

**Guiding principle:** Observe everything — alert for what matters — signals are gold.

---

### Phase 1: Proto Schema Rewrite
**Goal:** Replace all placeholder ContextL* messages in signal.proto with complete field definitions covering all inference engines, transformer architectures, and tokenizers. Align storage schema and SDK to the new contract.

**Status:** Planned

**Requirements:**
- REQ-P1-01: ContextL1 (hardware) — CPU, GPU, memory, disk, temperature, NUMA, power
- REQ-P1-02: ContextL2 (model) — model identity, architecture, inference engine, quantization, adapter, multimodal
- REQ-P1-03: ContextL3 (tokenizer) — token counts, chat template, density, prefix cache, injection indicators
- REQ-P1-04: ContextL4 (compute) — KV cache, batching, prefill/decode latency, speculative decoding, MoE routing
- REQ-P1-05: ContextL5 (output) — extend with missing fields (top_k, seed, stop_sequences, repetition)
- REQ-P1-06: ContextL6 (safety) — input/output safety scores, PII detection, jailbreak, injection
- REQ-P1-07: ContextL7 (retrieval) — extend with cache hit, citation tracking, index health
- REQ-P1-08: ContextL8 (agent) — extend with task hierarchy, memory ops, code execution
- REQ-P1-09: ContextL9 (API gateway) — method, path, status, auth, rate limiting, cost, quota
- REQ-P1-10: ContextL10 (application) — user session, business event, feature flags, A/B variant
- REQ-P1-11: ContextLDecision — add audit chain (policy_id, triggering_rule_id, triggering_signal_id, failed_open)
- REQ-P1-12: Regenerate Go stubs via make proto-generate
- REQ-P1-13: Align ClickHouse DDL in schema.go to new proto columns
- REQ-P1-14: Update sdk/signal_builder.py with builder methods for all new context types

---

### Phase 2: Build Stabilization
**Goal:** Fix all 14 failing packages so the codebase compiles cleanly.

**Status:** Not started

---

### Phase 3: API Completeness
**Goal:** Implement all missing backend endpoints required by the frontend — bedrock quality, no stubs, no TODOs.

**Status:** Planning
**Plans:** 5/5 plans complete

**Requirements:**
- REQ-P3-01: ClickHouse insert column sync — SignalsInsertColumns matches schema.go DDL
- REQ-P3-02: Layer status response shape — returns string enums and green/yellow/gray status
- REQ-P3-03: Trace response shape — returns spans[] with Span type, not signals[]
- REQ-P3-04: Query response shape — returns rows as objects with execution_time_ms
- REQ-P3-05: Rules routing fix — stub handlers delegate to real ServeListRules/ServeCreateRule
- REQ-P3-06: Apps CRUD — real handlers backed by PostgreSQL with API key generation
- REQ-P3-07: Health endpoint — check ClickHouse, PostgreSQL, Redis connectivity
- REQ-P3-08: Integration tests — verify all fixed/new endpoint response shapes

Plans:
- [x] 03-01-PLAN.md — ClickHouse column sync (Wave 0, blocks everything)
- [x] 03-02-PLAN.md — Response shape fixes for layer status, trace, query (Wave 1)
- [x] 03-03-PLAN.md — Rules routing fix + Apps CRUD (Wave 2)
- [x] 03-04-PLAN.md — Health endpoint expansion (Wave 2)
- [x] 03-05-PLAN.md — Integration tests (Wave 3)

---

### Phase 4: Detection Engine
**Goal:** Wire Tier 1/2/3 rule evaluation and Kairos policy decisions end-to-end with hybrid inline/async detection, PostgreSQL-backed rule hot-reload, fingerprinted alert lifecycle, bounded async queue with severity-based drop policy, circuit breaker, and full Prometheus instrumentation.

**Status:** Planning
**Plans:** 7/7 plans complete

Plans:
- [x] 04-01-PLAN.md — Fix double-write + restrict DetectionProcessor to Tier 1 + authoritative alerts migration 018 (Wave 0)
- [x] 04-02-PLAN.md — Async detection worker + circuit breaker + Prometheus metrics (Wave 1)
- [x] 04-03-PLAN.md — DB-backed rule hot-reload via MAX(version) polling (Wave 1)
- [x] 04-04-PLAN.md — Kairos conditional dispatch (requires_kairos, sampling, fail-open) (Wave 2)
- [x] 04-05-PLAN.md — Alert persistence: fingerprint dedup, lifecycle, JSONB context, trace linkage (Wave 2)
- [x] 04-06-PLAN.md — End-to-end wiring in cmd/argus/api.go (Wave 3)
- [x] 04-07-PLAN.md — Integration tests: signal → rule → alert path (Wave 4)

---

### Phase 5: Dashboard Integration
**Goal:** Frontend fully connected to real API data, WebSocket live feed working.

**Status:** In Progress
**Plans:** 1/6 complete (05-02)

**Requirements:**
- REQ-P5-01: HTTP service layer with typed API functions
- REQ-P5-02: API request/response types matching backend contracts
- REQ-P5-03: Login form with JWT token flow
- REQ-P5-04: Session validation and token refresh
- REQ-P5-05: Telemetry dashboard real data binding
- REQ-P5-06: WebSocket signal subscription
- REQ-P5-07: Trace flow real data binding
- REQ-P5-08: Hunting console query execution
- REQ-P5-09: Audit ledger real data binding

Plans:
- [ ] 05-01-PLAN.md — HTTP client service layer + typed API functions (Wave 1, planned)
- [x] 05-02-PLAN.md — Authentication integration + JWT + session validation (Wave 1, COMPLETE)
- [ ] 05-03-PLAN.md — Telemetry dashboard data binding + WebSocket (Wave 2, planned)
- [ ] 05-04-PLAN.md — Trace flow data binding (Wave 2, planned)
- [ ] 05-05-PLAN.md — Hunting console data binding (Wave 2, planned)
- [ ] 05-06-PLAN.md — Audit ledger data binding (Wave 2, planned)

---

### Phase 6: Security Hardening — Zero-Trust Auth & API Protection
**Goal:** Harden the platform to production security standards: wire RBAC enforcement to all routes, protect signal ingestion with API keys, add endpoint-level rate limiting, complete the secrets architecture, and fix auth gaps (CSRF, password policy, session management UI) — so that a security-focused platform is itself secure.

**Status:** Planned
**Plans:** 0/8 complete

**Requirements:**
- REQ-P6-01: Wire RBAC middleware (RequireRole/RequirePermission) to all protected routes — admin, analyst, viewer scoped
- REQ-P6-02: API key schema + issuance — scoped bearer tokens for SDK/machine identity (separate from user JWTs)
- REQ-P6-03: Authenticate signal ingestion endpoints (/v1/signals, /v1/signals/stream) via API key middleware
- REQ-P6-04: Endpoint rate limiting — wire existing Redis rate limiter to /auth/login (5/min per IP), /api/v1/query (60/min per user), SDK ingest (token bucket)
- REQ-P6-05: Secrets file architecture — argus.key encrypted file replacing raw env vars for JWT private key, DB credentials, API keys
- REQ-P6-06: Fix HIBP password breach check (response parsing broken in setup.go)
- REQ-P6-07: Complete TOTP 2FA flow — enroll, verify, backup codes endpoints
- REQ-P6-08: Session management — list active sessions, remote kill endpoint, UI for active devices
- REQ-P6-09: CSRF protection — double-submit cookie or signed CSRF token on all POST auth endpoints

Plans:
- [ ] 06-01-PLAN.md — RBAC wiring + context helpers (RequireAuth, UserIDFromContext) + HIBP fix (Wave 1)
- [ ] 06-02-PLAN.md — Rate limiting: /auth/login, /auth/refresh, /api/v1/query (Wave 2, after 06-01)
- [ ] 06-03-PLAN.md — API key schema migration 009 + CRUD endpoints + ValidateAPIKey (Wave 2, after 06-01)
- [ ] 06-04-PLAN.md — Secrets file architecture: internal/secrets + argus.key + CLI (Wave 3, after 06-01/02)
- [ ] 06-05-PLAN.md — Signal endpoint auth via X-Argus-API-Key + ingest token bucket (Wave 3, after 06-02/03)
- [ ] 06-06-PLAN.md — TOTP primitives: totp.go + migration 010_mfa + backup codes table (Wave 4, after 06-01/04)
- [ ] 06-07-PLAN.md — TOTP handlers: enroll/verify/disable/challenge + login MFA branch (Wave 5, after 06-06)
- [ ] 06-08-PLAN.md — Session management endpoints + CSRF double-submit + ExcludedPaths tightening (Wave 6, after 06-05/07)

---

## Requirements Index

| ID | Phase | Description |
|----|-------|-------------|
| REQ-P1-01 | 1 | ContextL1: hardware context fields |
| REQ-P1-02 | 1 | ContextL2: model anchor fields |
| REQ-P1-03 | 1 | ContextL3: tokenizer fields |
| REQ-P1-04 | 1 | ContextL4: compute/transformer internals |
| REQ-P1-05 | 1 | ContextL5: output extension |
| REQ-P1-06 | 1 | ContextL6: safety layer fields |
| REQ-P1-07 | 1 | ContextL7: retrieval extension |
| REQ-P1-08 | 1 | ContextL8: agent extension |
| REQ-P1-09 | 1 | ContextL9: API gateway fields |
| REQ-P1-10 | 1 | ContextL10: application context fields |
| REQ-P1-11 | 1 | ContextLDecision: audit chain extension |
| REQ-P1-12 | 1 | Proto stubs regenerated |
| REQ-P1-13 | 1 | ClickHouse DDL aligned |
| REQ-P1-14 | 1 | SDK signal_builder.py updated |
| REQ-P3-01 | 3 | ClickHouse insert column sync |
| REQ-P3-02 | 3 | Layer status response shape fix |
| REQ-P3-03 | 3 | Trace response shape fix |
| REQ-P3-04 | 3 | Query response shape fix |
| REQ-P3-05 | 3 | Rules routing fix |
| REQ-P3-06 | 3 | Apps CRUD implementation |
| REQ-P3-07 | 3 | Health endpoint expansion |
| REQ-P3-08 | 3 | Integration tests |
| REQ-P6-01 | 6 | RBAC middleware wired to all protected routes |
| REQ-P6-02 | 6 | API key schema + issuance for SDK/machine identity |
| REQ-P6-03 | 6 | Signal ingestion endpoints authenticated via API key |
| REQ-P6-04 | 6 | Endpoint rate limiting wired (auth, query, ingest) |
| REQ-P6-05 | 6 | Secrets file architecture (argus.key replaces raw env vars) |
| REQ-P6-06 | 6 | HIBP password breach check fixed |
| REQ-P6-07 | 6 | TOTP 2FA flow complete (enroll, verify, backup codes) |
| REQ-P6-08 | 6 | Session management UI (list, kill remote sessions) |
| REQ-P6-09 | 6 | CSRF protection on all POST auth endpoints |
