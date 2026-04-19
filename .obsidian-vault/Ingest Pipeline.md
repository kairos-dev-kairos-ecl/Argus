# Ingest Pipeline

> The signal hot path. From SDK POST to ClickHouse write in < 100ms p99.

## Entry Points (Receivers)

### HTTPReceiver
- File: `internal/ingest/receiver_http.go`
- Route: `POST /v1/signals`
- Auth: API key (header `X-Argus-Key`) validated against PostgreSQL `api_keys` table via `AuthValidator`
- Accepts: JSON body OR protobuf body (Content-Type detection)
- On success: `queue.Enqueue(signal)` → HTTP 202
- On queue full: HTTP 429
- Dedup check: `redis.CheckDedup(signal_id)` — drops duplicate within 15 min window

### GRPCReceiver
- File: `internal/ingest/receiver_grpc.go`
- Service: `IngestService/StreamSignals` (client-streaming gRPC, port 5001)
- Proto: `proto/argus/v1/service.proto`
- Accepts a stream of `ArgusSignal` messages, enqueues each, returns `IngestResponse { accepted, rejected }`

### OTLPReceiver
- File: `internal/ingest/receiver_otlp.go`
- Routes: `POST /v1/traces`, `POST /v1/metrics`
- Translates `ExportTraceServiceRequest` (OTLP spans) → `ArgusSignal` at L9/L10
- No auth required (standard OTLP push model)

### WebSocket
- File: `internal/ingest/receiver_ws.go`
- Route: `GET /v1/signals/stream`
- Subscribes to [[Ingest Pipeline#SignalBroadcaster]] fan-out
- Outbound only — clients receive live signal feed

---

## Queue

File: `internal/ingest/queue.go`

```go
type Queue struct {
    ch chan *v1.ArgusSignal  // buffered channel
}
capacity: 100,000 signals (ARGUS_INGEST_QUEUE_CAPACITY)
```

`Enqueue()` → non-blocking send; drops + logs if full
`Dequeue()` → blocking receive; used by WorkerPool

---

## WorkerPool

File: `internal/pipeline/workers.go`

```
N goroutines (default: GOMAXPROCS × 2 = 4 on quad-core)
Each goroutine:
  loop:
    signal = queue.Dequeue()  (blocks until signal available)
    chain.Process(signal)
    storage.Write(signal)     (ClickHouse BatchWriter)
    broadcaster.Publish(signal)  (WebSocket fan-out)
```

Backpressure: if BatchWriter is slow, goroutines block on Write — this blocks Dequeue — queue fills — HTTPReceiver returns 429.
Goroutine count is FIXED. No goroutine explosion under load.

---

## Pipeline Chain (7 Processors)

File: `internal/pipeline/interface.go` — `Processor` interface + `Chain`

Each processor implements: `Process(ctx, signal) error`
Chain is serial — each processor sees the (potentially mutated) signal from the previous.

### 1. SchemaValidator
File: `internal/pipeline/validator.go`

Rejects signals missing required fields:
- `signal_id` empty
- `trace_id` empty
- `layer` == LAYER_UNSPECIFIED (0)
- `source.app_id` empty

On rejection: increments `pipeline_rejected_total{reason}`, signal is dropped.

### 2. Normalizer
File: `internal/pipeline/normalizer.go`

Mutations applied:
- `provider.name` → lowercase + trim
- `timestamp` 0 → set to `time.Now()` (UTC nanoseconds)
- `ingested_at` → always set to server receipt time
- `severity` out of range → clamp to INFO
- `layer` out of range → reject

### 3. CorrelationTagger
File: `internal/pipeline/correlator.go`

Uses Redis ZSET `trace:{trace_id}`:
1. `ZADD trace:{trace_id} score={now_ms} member={signal_id}` + EXPIRE 30s
2. `ZRANGEBYSCORE ... [now-5000 now]` → get signals in last 5s of same trace
3. Appends to `signal.related_signals[]`

Connects: [[Storage Layer#Redis]] `trace:*` keys

### 4. BaselineScorer
File: `internal/pipeline/baseline.go`

1. Look up profile: `ProfileStore.GetProfile(app_id, layer, category)`
   - Redis cache first (`baseline:{app}:{layer}:{cat}`) → TTL 5 min
   - Fallback: PostgreSQL `baseline_profiles` table
2. Compute z-score: `(value - mean) / stddev`
   - `value` = `signal.duration_ms` (primary) or layer-specific metric
   - Stddev = 0 → z-score = 0 (no NaN/Inf)
3. Set `signal.enrichment.baseline_deviation = z_score`

Connects: [[Baseline Engine]], [[Storage Layer#Redis]], [[Storage Layer#PostgreSQL]]

### 5. Enricher
File: `internal/pipeline/enricher.go` + `geoip.go`

GeoIP enrichment using MaxMind geoip2 database:
1. Extract IP from `signal.source.instance_id` or L9 `client_ip`
2. Cache check: `redis.GetGeoIP(ip)` → TTL 24h
3. On miss: `geoip2.Reader.City(ip)` → country, city, lat/lon, ASN
4. Set `signal.enrichment.geo.*`
5. Cache result

Non-fatal: if GeoIP DB missing or IP private → enrichment skipped, pipeline continues.

### 6. DetectionProcessor
File: `internal/pipeline/detection.go`

Calls `DetectionEngine.Evaluate(signal)` → see [[Detection Engine]]
On match: calls `AlertRouter.WriteAlert(alert)` → see [[Notify & Alerting]]
Non-fatal: detection failure logged, pipeline continues.

---

## SignalBroadcaster

File: `internal/ingest/broadcaster.go`

Fan-out hub for WebSocket clients:
```
Publish(signal) → iterate all subscribers → non-blocking send
Subscribe()     → returns read-only channel
Unsubscribe()   → removes channel from subscriber map
```
Slow clients are dropped (non-blocking send with select + default).

---

## AuthValidator

File: `internal/ingest/auth.go`

Validates API keys for HTTP ingest:
1. Hash the provided key (SHA256)
2. Query PostgreSQL: `SELECT app_id FROM api_keys WHERE key_hash=$1 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > now())`
3. Cache result in memory (5-min TTL) to avoid PG hit per signal

---

## File Map

| File | Component |
|------|-----------|
| `internal/ingest/receiver_http.go` | HTTPReceiver |
| `internal/ingest/receiver_grpc.go` | GRPCReceiver |
| `internal/ingest/receiver_otlp.go` | OTLPReceiver |
| `internal/ingest/receiver_ws.go` | WebSocket handler |
| `internal/ingest/queue.go` | In-memory queue |
| `internal/ingest/auth.go` | AuthValidator |
| `internal/ingest/broadcaster.go` | SignalBroadcaster |
| `internal/pipeline/workers.go` | WorkerPool |
| `internal/pipeline/interface.go` | Processor interface + Chain |
| `internal/pipeline/validator.go` | SchemaValidator |
| `internal/pipeline/normalizer.go` | Normalizer |
| `internal/pipeline/correlator.go` | CorrelationTagger |
| `internal/pipeline/baseline.go` | BaselineScorer |
| `internal/pipeline/enricher.go` | Enricher (GeoIP) |
| `internal/pipeline/geoip.go` | GeoIPEnricher (MaxMind) |
| `internal/pipeline/detection.go` | DetectionProcessor |
| `internal/pipeline/router.go` | SignalRouter (output routing) |
| `internal/storage/clickhouse.go` | BatchWriter (final sink) |
