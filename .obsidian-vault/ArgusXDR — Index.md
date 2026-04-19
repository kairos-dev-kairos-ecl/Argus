# ArgusXDR — Master Index

> XDR platform for LLM-integrated systems. Full-stack signal coverage L1–L10.

## Navigation

```
ArgusXDR
├── [[Signal — ArgusSignal Schema]]      ← The core data atom
├── [[Storage Layer]]                    ← ClickHouse · PostgreSQL · Redis
├── [[Ingest Pipeline]]                  ← Queue → Chain → Write
├── [[API Routes]]                       ← HTTP · gRPC · WebSocket · OTLP
├── [[Detection Engine]]                 ← Tier 1 · Tier 2 · Tier 3 · Kairos
├── [[Notify & Alerting]]               ← Routing · Dispatch · Adapters
├── [[Auth & RBAC]]                      ← JWT · Roles · Sessions · Audit
├── [[Baseline Engine]]                  ← Z-score · Profiles · Background
├── [[Web Dashboard]]                    ← React · Zustand · ECharts
├── [[SDK]]                              ← Python · TypeScript
├── [[Config & Environment]]             ← Viper · ARGUS_* env vars
└── [[Deployment]]                       ← Docker · Dockerfile · Makefile
```

## Signal Flow (End-to-End)

```
SDK (Python/TS)
  │  POST /v1/signals  (JSON or protobuf)
  │  gRPC StreamSignals
  │  OTLP POST /v1/traces
  ▼
[[Ingest Pipeline#HTTPReceiver / GRPCReceiver / OTLPReceiver]]
  │  AuthValidator ← PostgreSQL api_keys table
  ▼
ingest.Queue  (buffered channel, cap 100 000)
  ▼
WorkerPool  (2 goroutines, GOMAXPROCS×2)
  ▼
pipeline.Chain (sequential)
  1. [[Ingest Pipeline#SchemaValidator]]
  2. [[Ingest Pipeline#Normalizer]]
  3. [[Ingest Pipeline#CorrelationTagger]]   ← Redis sorted sets
  4. [[Ingest Pipeline#BaselineScorer]]      ← Redis + PostgreSQL profiles
  5. [[Ingest Pipeline#Enricher]]            ← MaxMind GeoIP
  6. [[Ingest Pipeline#DetectionProcessor]]  ← [[Detection Engine]]
  ▼                                          │
[[Storage Layer#ClickHouse BatchWriter]]     │ alert match
  signals table                             ▼
                                     [[Notify & Alerting#AlertRouter]]
                                       dedup → upsert → incident → dispatch
                                       Slack · PagerDuty · Email · Webhook
  ▼
[[Web Dashboard#SignalStream]]  ← WebSocket /v1/signals/stream
```

## Tech Stack at a Glance

| Layer | Technology |
|-------|-----------|
| Core runtime | Go 1.24 |
| HTTP router | chi/v5 |
| CLI/Config | cobra + viper |
| Logging | zap |
| Metrics | prometheus/client_golang |
| Signal protocol | protobuf v2 + gRPC v1.71 |
| Time-series DB | ClickHouse 24 (ReplacingMergeTree) |
| Relational DB | PostgreSQL 16 (pgx/v5) |
| Cache/ephemeral | Redis 7.2 (go-redis/v9) |
| Migrations | golang-migrate/v4 |
| Auth | RS256 JWT (golang-jwt/v5) + bcrypt |
| GeoIP | MaxMind geoip2-golang |
| Dashboard | React 19 + Vite + TanStack Query + Zustand + ECharts |
| SDK | Python (httpx + protobuf) · TypeScript |
| Container | Docker multi-stage (golang:1.24-alpine → alpine:3.19) |
