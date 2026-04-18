# Phase 4: Detection Engine - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-19
**Phase:** 04-detection-engine
**Areas discussed:** Pipeline integration mode, Alert data model & lifecycle, Rule loading strategy, Kairos integration contract, Back pressure & enterprise resilience
**Context:** User provided Falcon Onum validation spec (ARGUS_ONUM_VALIDATION_SPEC.md) as quality bar reference. Objective: Argus should match enterprise observability platform depth for LLM observability.

---

## Pipeline Integration Mode

| Option | Description | Selected |
|--------|-------------|----------|
| Inline only | All rules in DetectionProcessor, blocking | |
| Async only | All detection post-storage | |
| Hybrid (inline tagging + async decisions) | Cheap deterministic rules inline (<50ms), complex rules async | ✓ |

**User's choice:** Hybrid. Inline = signal tagging (Tier 1 field-match only). Async = decision making (Tier 2/3, Kairos). DetectionProcessor must be non-blocking by default.

**Notes:** "Design principle: Inline detection = signal tagging, async detection = decision making."

---

## Alert Data Model & Lifecycle

| Option | Description | Selected |
|--------|-------------|----------|
| Simple open/closed | Two states, minimal tracking | |
| Full lifecycle with fingerprinting | OPEN→ACK→RESOLVED→SUPPRESSED, hash-based dedup, trace linkage, JSONB context | ✓ |
| External alertmanager delegation | Offload to Prometheus Alertmanager | |

**User's choice:** Full lifecycle. OPEN → ACKNOWLEDGED → RESOLVED → SUPPRESSED. Fingerprint = `hash(rule_id + entity + normalized_payload)` — non-optional. Trace linkage to `trace_id` and `signal_id[]` mandatory. Context as JSONB.

**Notes:** "If this is weak, your platform becomes unusable fast." Fingerprinting prevents alert storms. Rigid schemas break when rules evolve.

---

## Rule Loading Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| YAML source of truth | Files on disk, restart to reload | |
| PostgreSQL source of truth | DB authoritative, YAML for seeding only, hot reload | ✓ |
| External rule registry | Separate service | |

**User's choice:** Hybrid with PostgreSQL as authoritative source. YAML = bootstrap/version-control only. PostgreSQL `detection_rules` = runtime source. Hot reload every 30–60s (or event-driven on write). Rules compiled to in-memory evaluators — not interpreted raw JSON per request.

**Notes:** "If you keep YAML as source of truth, you'll regret it." Rule versioning via `version` integer column — compare max(version) to detect changes.

---

## Kairos Integration Contract

| Option | Description | Selected |
|--------|-------------|----------|
| Synchronous, every signal | Kairos in critical path | |
| Async, every signal | Off critical path but wasteful | |
| Async, conditional (rule-triggered + sampling) | Kairos as augmentation layer | ✓ |

**User's choice:** Async, conditional. Fail-open always. Kairos called only when: rule fired + `requires_kairos: true`, OR sampling (1–5%), OR high-risk tenant/model. Timeout = no signal (not error). "Fail-closed will break production traffic."

**Notes:** Kairos = augmentation layer, not a dependency. System must operate fully without Kairos.

---

## Back Pressure & Enterprise Resilience

| Option | Description | Selected |
|--------|-------------|----------|
| Defer to infrastructure phase | Ship detection engine first | |
| Minimal (bounded queue only) | Cap queue size, no policies | |
| Full enterprise resilience | Bounded queues + drop policies + metrics + circuit breakers | ✓ |

**User's choice:** Full enterprise resilience — non-negotiable. Max queue 10k, drop LOW first, preserve CRITICAL always. All 5 throughput metrics required. Circuit breaker: CLOSED → OPEN (degrade to Tier 1 only, disable Kairos, sample 10%) → HALF-OPEN.

**Notes:** "If this is P0 and you defer this — you're building a demo, not a platform." The Onum validation spec marks back pressure as P0.

---

## Deferred Ideas

- SSO/2FA/SAML/OIDC auth stack
- Secrets architecture and key rotation
- SDK bearer token issuance
- Splunk / PagerDuty sink connectors
- Hero visualization (ECL State Distribution Panel)
- Multitenancy / tenant-scoped RBAC
