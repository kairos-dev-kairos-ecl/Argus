# Detection Engine

> Rules-as-configuration. Three tiers of signal evaluation, plus an optional external policy engine.

## Architecture

```
DetectionEngine.Evaluate(signal)
  │
  ├─ Tier 1: Static rule matching      (pure signal fields)
  ├─ Tier 2: Baseline deviation        (requires enriched z-score)
  ├─ Tier 3: Temporal frequency        (requires Redis sorted sets)
  └─ Kairos: External policy engine    (optional HTTP call)
```

Files: `internal/detection/engine/`

---

## Rules

### Structure
File: `internal/detection/engine/rule.go`

```yaml
id: rule-001
name: High CPU Spike
description: CPU utilization exceeds threshold
enabled: true
tier: 1                        # 1 | 2 | 3
layer: L1_HARDWARE             # Layer enum name
category_prefix: "infra.cpu"   # matches category starting with this
severity_gte: MEDIUM           # minimum severity to match
baseline_deviation_gte: 3.0    # Tier 2: z-score threshold
count_gte: 5                   # Tier 3: event count threshold
window_seconds: 60             # Tier 3: time window
alert_severity: HIGH
alert_title: "CPU spike detected on {{.AppID}}"
alert_description: "..."
```

### Loading
- File: `internal/detection/engine/loader.go`
- `LoadRuleFromFile(path)` → reads YAML → `Rule` struct
- `LoadRulesFromDirectory(dir)` → loads all `.yaml` files in directory
- Default built-in rules dir: `internal/rules/built-in/` (15 rules, copied into Docker image)
- Rules also stored in PostgreSQL `detection_rules` table (synced on load)
- In-memory store: `RuleStore` (thread-safe map)

---

## Tier 1 — Static Matching

File: `internal/detection/engine/tier1.go`

Evaluates three field conditions (all must match):
1. `signal.layer == rule.layer` (if rule.layer set)
2. `strings.HasPrefix(signal.category, rule.category_prefix)` (if set)
3. `signal.severity >= rule.severity_gte` (if set)

Fast path — no external dependencies. Runs on every signal.

---

## Tier 2 — Baseline Deviation

File: `internal/detection/engine/tier2.go`

Prerequisite: [[Ingest Pipeline#BaselineScorer]] must have set `signal.enrichment.baseline_deviation`

Condition: `signal.enrichment.baseline_deviation >= rule.baseline_deviation_gte`

Typical threshold: `3.0` (3 standard deviations = statistical anomaly)

No additional lookups — uses enrichment already set in pipeline.

---

## Tier 3 — Temporal Frequency

File: `internal/detection/engine/tier3.go`

Detects bursts: "N events of type X in Y seconds"

1. Key: `det:t3:{rule_id}:{app_id}:{category}`
2. `ZADD key score={now_unix_ms} member={signal_id}`
3. `ZREMRANGEBYSCORE key -inf {now_ms - window_ms}` (trim old events)
4. `ZCARD key` → current count in window
5. If count >= rule.count_gte → trigger alert

Uses: [[Storage Layer#Redis]] sorted sets

---

## Alert Generation

On any tier match → `AlertRouter.WriteAlert(alert)`:
- `alert.fingerprint` = SHA256(`rule_id + app_id + layer + category`)
- Dedup: `redis.SetNX("dedup:{fingerprint}")` — skip if seen in last 10 min
- Upsert: PostgreSQL `alerts` table (increment signal_count if exists)
- Incident correlation: if ≥ 3 alerts share trace_id within 10 min → create/update `incidents` row
- Then: [[Notify & Alerting#AlertDispatcher]] sends to configured channels

---

## Kairos — External Policy Engine

Files: `internal/detection/kairos/`

Optional integration with an external Kairos policy engine (HTTP).

### Client (`client.go`)
```
POST http://kairos-host/policy/evaluate
Body: { signal_id, layer, category, severity, context, enrichment }
Response: { decision: ALLOW|DENY|REVIEW, confidence, reasoning, recommended_action }
```

### Evaluator (`evaluator.go`)
Wraps the Kairos client call, handles timeouts and errors (non-fatal).

### Signal Builder (`signal_builder.go`)
Converts Kairos response into a `ContextLDecision` signal:
- `Layer = L_DECISION`
- `category = "decision.policy"`
- Populates `ContextLDecision` with decision, confidence, reasoning

The decision signal is emitted back into the pipeline for logging in ClickHouse.

---

## Built-in Rules (15 rules)

Located at: `internal/rules/built-in/`
Copied into Docker image via Dockerfile.

Examples:
- `cpu-spike.yaml` — L1, infra.cpu, deviation ≥ 3σ
- `memory-pressure.yaml` — L1, infra.memory, severity ≥ MEDIUM
- `model-hash-change.yaml` — L2, model.load, Tier 1 static
- `token-truncation.yaml` — L3, tokenizer.truncation, count ≥ 5 in 60s
- `kv-cache-miss.yaml` — L4, inference.kv_cache, deviation ≥ 2σ
- `output-refusal.yaml` — L5, output.refusal, Tier 1
- `safety-block.yaml` — L6, safety.block, severity ≥ HIGH
- `low-retrieval-score.yaml` — L7, retrieval.low_score, Tier 1
- `permission-escalation.yaml` — L8, agent.permission, CRITICAL
- `rate-limit-burst.yaml` — L9, gateway.rate_limited, Tier 3 count ≥ 10 in 30s
- `session-anomaly.yaml` — L10, app.session, temporal

---

## File Map

| File | Component |
|------|-----------|
| `internal/detection/engine/engine.go` | DetectionEngine.Evaluate() |
| `internal/detection/engine/rule.go` | Rule struct + YAML tags |
| `internal/detection/engine/loader.go` | File/directory rule loading |
| `internal/detection/engine/store.go` | RuleStore (in-memory map) |
| `internal/detection/engine/tier1.go` | Static field matching |
| `internal/detection/engine/tier2.go` | Baseline deviation matching |
| `internal/detection/engine/tier3.go` | Temporal frequency (Redis) |
| `internal/detection/kairos/client.go` | Kairos HTTP client |
| `internal/detection/kairos/evaluator.go` | Kairos integration |
| `internal/detection/kairos/signal_builder.go` | ContextLDecision builder |
| `internal/rules/built-in/*.yaml` | 15 built-in detection rules |
