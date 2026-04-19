# Baseline Engine

> Background statistical profiling. Never blocks the ingest hot path.

## Purpose

Computes per-(app_id, layer, category) statistical profiles:
- Mean, standard deviation, min, max of signal metrics
- Used by [[Ingest Pipeline#BaselineScorer]] to compute z-scores
- Enables [[Detection Engine#Tier 2 — Baseline Deviation]] anomaly detection

## Architecture

```
BaselineEngine (background goroutine, runs every 10 minutes)
  │
  ├─ Query ClickHouse: aggregate metrics per (app_id, layer, category)
  │    WHERE timestamp > now() - history_window (default 24h)
  │    HAVING count(*) >= 100  (min samples for valid profile)
  │
  ├─ ProfileCalculator.Compute(samples)
  │    → mean = sum / count
  │    → stddev = sqrt(Σ(x-mean)² / count)
  │    → stddev = 0 → clamp to 0 (prevents NaN z-scores)
  │
  ├─ ProfileStore.StoreProfile(profile)
  │    ├─ Redis: SET baseline:{app}:{layer}:{cat} TTL 5 min  (fast path cache)
  │    └─ PostgreSQL: UPSERT baseline_profiles (durable)
  │
  └─ sleep 10 min → repeat
```

## Files

| File | Component |
|------|-----------|
| `internal/baseline/engine.go` | BaselineEngine — background ticker, ClickHouse query |
| `internal/baseline/calculator.go` | ProfileCalculator — mean/stddev/z-score math |
| `internal/baseline/store.go` | ProfileStore — Redis cache + PostgreSQL persistence |
| `internal/baseline/config.go` | BaselineConfig — intervals, thresholds, min samples |

## ProfileStore Read Path (Hot Path)

Used by `BaselineScorer` in the ingest pipeline (per signal):

```
GetProfile(app_id, layer, category)
  1. Redis GET baseline:{app_id}:{layer}:{category}
     → hit: return cached profile (deserialized JSON)
  2. miss: PostgreSQL SELECT from baseline_profiles
     → found: write-back to Redis (TTL 5 min), return
  3. not found: return nil (no baseline yet — z-score = 0)
```

## Z-Score Computation

File: `internal/baseline/calculator.go`

```go
func ComputeZScore(value, mean, stddev float64) float64 {
    if stddev == 0 {
        return 0  // avoid NaN/Inf
    }
    return (value - mean) / stddev
}
```

The `value` used depends on layer:
- Default: `signal.duration_ms`
- L1: `ctx_l1_cpu_percent`
- L5: `ctx_l5_tps` (tokens per second)

## Pitfall Prevention

| Pitfall | Prevention |
|---------|-----------|
| Blocks ingest hot path | Background goroutine only — never called synchronously during ingest |
| Redis memory bloat | All baseline keys have 5-min TTL |
| NaN z-scores | stddev=0 → z-score=0 (clamped) |
| Stale baselines across restarts | PostgreSQL is source of truth; Redis is warm cache |
| Underfitting (too few samples) | `HAVING count(*) >= 100` gates profile computation |

## Connections

- [[Storage Layer#ClickHouse]] — source data for profile computation
- [[Storage Layer#PostgreSQL]] → `baseline_profiles` table — durable storage
- [[Storage Layer#Redis]] → `baseline:{app}:{layer}:{cat}` — fast read cache
- [[Ingest Pipeline#BaselineScorer]] — consumes profiles in hot path
- [[Detection Engine#Tier 2 — Baseline Deviation]] — uses z-score from enrichment
