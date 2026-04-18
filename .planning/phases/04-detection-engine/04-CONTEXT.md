# Phase 4: Detection Engine - Context

**Gathered:** 2026-04-19
**Status:** Ready for planning

<domain>
## Phase Boundary

Wire Tier 1/2/3 rule evaluation and Kairos policy decisions into the live signal pipeline end-to-end — so that every ingested signal is evaluated against rules, alerts are persisted with proper lifecycle, rules hot-reload from PostgreSQL, and Kairos decisions augment (not block) the signal path. Back pressure and circuit breakers are non-negotiable in this phase.

**Out of scope:** New rule authoring UI, SSO/2FA auth stack, Splunk/PagerDuty sinks, multitenancy.

</domain>

<decisions>
## Implementation Decisions

### Pipeline Integration Mode
- **D-01:** Hybrid detection — NOT inline-only, NOT fully async. Two distinct paths:
  - **Inline (fast path):** Only cheap, deterministic rules run inside `DetectionProcessor`. Examples: threshold checks, layer/category/severity field matches (Tier 1). Signal gets tagged/flagged but DetectionProcessor **must not block** — <50ms budget, no I/O.
  - **Async (default for decisions):** Complex rules (Tier 2 baseline deviation, Tier 3 temporal frequency, ML scoring, multi-signal correlation) dispatch to a bounded async queue post-ingestion. Zero impact on request latency.
- **D-02:** Design principle: **inline detection = signal tagging, async detection = decision making**. Inline path enriches the trace. Async path creates alerts.
- **D-03:** `DetectionProcessor` in the 7-stage pipeline chain handles inline only. Async dispatch happens after storage write (signal already persisted before detection decision).

### Alert Data Model & Lifecycle
- **D-04:** Alert lifecycle: `OPEN → ACKNOWLEDGED → RESOLVED → SUPPRESSED` (four states only)
  - `OPEN`: auto-created when a rule fires
  - `ACKNOWLEDGED`: human has seen it (sets `acknowledged_at`, `acknowledged_by`)
  - `RESOLVED`: action taken / closed (sets `resolved_at`)
  - `SUPPRESSED`: auto-muted by dedup / noise control
- **D-05:** **Fingerprinting is non-optional.** Every alert gets: `hash(rule_id + entity + normalized_payload)`. Prevents alert storms. Duplicate fingerprint = update existing open alert (increment count), not create new.
- **D-06:** **Trace linkage is mandatory.** Every alert must reference: `trace_id`, `signal_id[]` (the triggering signals). Enables root-cause navigation from alert → trace → signals.
- **D-07:** Alert `context` stored as **JSONB** in PostgreSQL. No rigid schema for rule-specific payload — rules evolve faster than DB migrations. Core fields (`id`, `rule_id`, `fingerprint`, `state`, `severity`, `trace_id`, timestamps) are typed columns; everything else goes in `context`.

### Rule Loading Strategy
- **D-08:** **PostgreSQL `detection_rules` table is the authoritative source of truth.** YAML files are for seeding defaults and version control only — loaded once at bootstrap, then PostgreSQL takes over.
- **D-09:** **Hot reload is required.** Strategy: polling (30–60s TTL) OR event-driven (preferred — signal from API write to `detection_rules`). In-memory `RuleStore` is refreshed on version change or TTL expiry without restart.
- **D-10:** Rules must be compiled into **in-memory evaluators** — not interpreted from raw JSON on every signal. Tier 1 rules compile to field-match predicates, Tier 2 to threshold comparators, Tier 3 to temporal window configs. Raw rule structs are only parsed during load/reload.
- **D-11:** Rule versioning: each row in `detection_rules` has a `version` integer. Hot reload compares max(version) against last-loaded version to detect changes efficiently.

### Kairos Integration Contract
- **D-12:** **Async by default.** Kairos is called from the async detection queue, never from the inline signal path. Sync Kairos calls are opt-in only for explicitly high-risk tier configurations.
- **D-13:** **Fail-open always.** If Kairos is unreachable: treat as no enrichment, emit alert with `kairos_decision: null`, log + increment metric. **Never fail-closed** — fail-closed breaks production traffic.
- **D-14:** Timeout behavior: treat as no signal from Kairos (not an error that blocks the alert). Log the timeout, increment `kairos_timeout_total` metric.
- **D-15:** **Kairos is called only when:**
  1. A rule triggered AND the rule's action requires Kairos enrichment (`requires_kairos: true` flag on rule), OR
  2. Signal falls within a sampling window (1–5% baseline traffic for policy baseline), OR
  3. Signal is from a high-risk tenant or model (configurable list)
  - Pattern: Signal → lightweight rule evaluation → (conditional) Kairos → alert decision
  - Kairos = **augmentation layer**, not a dependency. System operates fully without it.

### Back pressure & Enterprise Resilience
- **D-16:** **Bounded alert/detection queue — non-negotiable.** `max_size = 10,000`. Drop policy when full: drop `LOW` severity first, preserve `CRITICAL` always, optionally sample `MEDIUM` at 50%.
- **D-17:** **Drop policies must be explicit:** lowest severity dropped first under pressure. Any dropped alert increments `argus_detection_dropped_total{severity="low"}`. Dropped alerts are logged (not silently lost).
- **D-18:** **Throughput metrics (all required at launch):**
  - `argus_detection_latency_seconds` (histogram, p50/p95/p99) — per-signal detection time
  - `argus_detection_queue_depth` (gauge) — current async queue length
  - `argus_rule_evaluations_total` (counter, by tier) — rule evaluation rate
  - `argus_alert_created_total` (counter, by severity/rule) — alert creation rate
  - `argus_detection_dropped_total` (counter, by severity) — dropped alert count
- **D-19:** **Circuit breakers required.** When detection system lags (queue depth > 80% of max OR p99 latency > 500ms):
  1. Degrade to minimal rules only (Tier 1 inline, no Tier 2/3 async)
  2. Disable Kairos calls
  3. Switch async evaluation to sampling mode (configurable %, default 10%)
  Circuit breaker auto-recovers when metrics normalize. State: CLOSED → OPEN → HALF-OPEN.

### Claude's Discretion
- Circuit breaker library choice (implement from scratch vs use existing Go circuit breaker package)
- Exact polling interval for rule hot-reload (within 30-60s window)
- Whether `SUPPRESSED` alerts are soft-deleted or kept with state flag
- Async queue implementation (channel-based vs ring buffer)
- YAML seed file format and location (e.g., `rules/built-in/*.yaml`)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Detection Engine (existing code)
- `internal/detection/engine/engine.go` — `DetectionEngine.Evaluate()`, the top-level evaluation entrypoint
- `internal/detection/engine/tier1.go` — Tier 1 field-match implementation
- `internal/detection/engine/tier2.go` — Tier 2 baseline deviation implementation
- `internal/detection/engine/tier3.go` — Tier 3 temporal frequency + `TemporalStore` interface
- `internal/detection/engine/store.go` — In-memory `RuleStore` (thread-safe, add/remove/enabled)
- `internal/detection/engine/loader.go` — YAML rule loader (`LoadRulesFromDirectory`)
- `internal/detection/engine/rule.go` — `Rule`, `Conditions`, `Action` structs — extend here for `requires_kairos` flag

### Kairos Integration (existing code)
- `internal/kairos/client.go` — `Client` HTTP client, `EvaluationRequest`, `PolicyDecision`
- `internal/kairos/evaluator.go` — `Evaluator` with caching + fallback behavior
- `internal/kairos/policy.go` — `Policy`, `PolicyRule`, `ValidatePolicy`
- `internal/kairos/signal_builder.go` — Builds `EvaluationRequest` from `ArgusSignal`

### Pipeline (existing code)
- `internal/pipeline/` — 7-stage chain; `DetectionProcessor` is the inline detection slot
- `internal/ingest/handler_stubs.go` — Rules CRUD handlers (Phase 3: DB-backed)
- `internal/ingest/receiver_query.go` — `QueryHandler` with `SetPool` (Phase 3)

### Storage (existing code)
- `internal/storage/clickhouse.go` — Signal persistence (Phase 3: 246-column insert)
- `internal/storage/postgres.go` — PostgreSQL pool; alerts table needs migration here
- `internal/storage/schema.go` — `SignalsTableDDL`; alerts DDL will live alongside

### Onum Validation Spec (external reference)
- `D:\Downloads\ARGUS_ONUM_VALIDATION_SPEC.md` — Section 10 Gap Priority Matrix: P0 gaps (secrets, SDK auth, back pressure), P1 gaps (RBAC, hero viz). Phase 4 addresses back pressure P0 and lays groundwork for P1 detection quality.

### Database (Phase 3 built)
- `detection_rules` table in PostgreSQL — source of truth for rules (Phase 3 wired CRUD)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `engine.DetectionEngine` — fully functional, tests passing; needs async dispatch wiring, not rewriting
- `engine.RuleStore` — thread-safe in-memory store; needs hot-reload trigger added
- `engine.LoadRulesFromDirectory` — YAML loader works; needs DB-backed reload alongside it
- `kairos.Evaluator` — caching + fail-open already implemented; needs to be called from async worker, not inline
- `kairos.Client` — HTTP client ready; sampling/conditional call logic needs adding
- `internal/ingest/alert_router.go` — `AlertRouter` type exists; needs lifecycle states and fingerprinting

### Established Patterns
- Prometheus metrics: `internal/metrics/` — existing counters/histograms; new detection metrics follow same pattern
- Structured logging: `go.uber.org/zap` throughout — use same logger injection
- PostgreSQL: `pgx/v5` pool — migrations in `internal/storage/migrations/`
- Channel-based queues: ingest queue uses `chan *v1.ArgusSignal` with capacity — same pattern for async detection queue
- Context propagation: all handlers use `context.Context` — carry through detection path

### Integration Points
- `internal/pipeline/DetectionProcessor` — inline tagging slot (call `engine.Evaluate()` for Tier 1 only)
- After `BatchWriter` in pipeline — async dispatch point (post-storage, non-blocking)
- `cmd/argus/api.go` — startup wiring (inject detection engine + async worker into pipeline)
- `internal/storage/postgres.go` — add `alerts` table migration and insert/query methods
- `internal/ingest/receiver_query.go` — alerts list/get endpoints already scaffolded via `AlertRouter`

</code_context>

<specifics>
## Specific Ideas

- **"Inline detection = signal tagging, async detection = decision making"** — this is the core design principle; all implementation choices should respect this boundary
- **"Fail-closed will break production traffic"** — Kairos must never be in the critical path
- **Alert storm prevention via fingerprinting** is non-negotiable — same as how Prometheus deduplicates alert groups
- **"If you keep YAML as source of truth, you'll regret it"** — PostgreSQL is authoritative; YAML is bootstrap only
- **"If this is P0 and you defer this — you're building a demo, not a platform"** — back pressure is included in Phase 4, not deferred
- Circuit breaker pattern: CLOSED (normal) → OPEN (degraded, sampling only) → HALF-OPEN (recovering) — standard Go circuit breaker state machine

</specifics>

<deferred>
## Deferred Ideas

- SSO / 2FA / SAML / OIDC auth stack — Onum P1, scoped to a dedicated auth phase (Phase 5+)
- Splunk / PagerDuty / Teams sink connectors — Onum P1, blocked by secrets architecture
- Secrets architecture (vault, key rotation, TTL) — Onum P0, own phase before sink connectors ship
- SDK agent bearer token issuance — Onum P0, own phase
- Hero visualization (ECL State Distribution Panel) — Onum P1, Phase 5 dashboard work
- Multitenancy / tenant-scoped roles — architecture decision needed first
- Pipeline visual editor — Phase 5+
- PII anonymization / DLP integration — Phase 6+
- Real-time signal throughput view in dashboard — Phase 5 frontend work

</deferred>

---

*Phase: 04-detection-engine*
*Context gathered: 2026-04-19*
