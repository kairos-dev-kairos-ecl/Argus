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
**Plans:** 3/5 plans executed

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
- [ ] 03-05-PLAN.md — Integration tests (Wave 3)

---

### Phase 4: Detection Engine
**Goal:** Tier 1/2/3 rule evaluation + Kairos integration working end-to-end.

**Status:** Not started

---

### Phase 5: Dashboard Integration
**Goal:** Frontend fully connected to real API data, WebSocket live feed working.

**Status:** Not started

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
