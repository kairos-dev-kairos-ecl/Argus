# API Routes

> All HTTP + gRPC endpoints. Port 8080 (HTTP) + 5001 (gRPC).

## Middleware Stack (applied globally via chi)

```
RequestID → RealIP → Recoverer → Logger → PrometheusMiddleware
  → AuthMiddleware (JWT Bearer — skipped for ingest + health + metrics)
```

---

## Ingest Endpoints (no JWT required — API key only)

| Method | Path | Auth | File | Description |
|--------|------|------|------|-------------|
| POST | `/v1/signals` | API Key | `receiver_http.go` | Emit signals (JSON or protobuf) |
| POST | `/v1/traces` | none | `receiver_otlp.go` | OTLP trace spans |
| POST | `/v1/metrics` | none | `receiver_otlp.go` | OTLP metrics |
| GET | `/v1/signals/stream` | JWT | `receiver_ws.go` | WebSocket live stream |

---

## Query Endpoints (JWT required)

| Method | Path | Role | File | Description |
|--------|------|------|------|-------------|
| GET | `/v1/signals` | viewer+ | `receiver_query.go` | Paginated signal query |
| GET | `/v1/schema/signals` | viewer+ | `receiver_query.go` | Column schema |
| GET | `/api/v1/layers/status` | viewer+ | `receiver_query.go` | Per-layer counts (last 5 min) |
| GET | `/api/v1/traces/{traceId}` | viewer+ | `receiver_query.go` | All signals for a trace |
| POST | `/api/v1/query` | analyst+ | `receiver_query.go` | Raw SQL (read-only, 5000 row max) |

### `/v1/signals` Query Parameters
```
app_id     required  string
layer      optional  int (1-10)
category   optional  string prefix
severity   optional  int (1-5)
start      optional  RFC3339 timestamp
end        optional  RFC3339 timestamp
cursor     optional  pagination cursor (base64 encoded ts+id)
limit      optional  int (default 100, max 1000)
```

Response: `{ signals: [...], next_cursor: "..." }`

---

## Rules (JWT required)

| Method | Path | Role | File |
|--------|------|------|------|
| GET | `/api/v1/rules` | viewer+ | `handler_rules.go` |
| POST | `/api/v1/rules` | admin | `handler_rules.go` |
| GET | `/api/v1/rules/{id}` | viewer+ | `handler_rules.go` |
| PUT | `/api/v1/rules/{id}` | admin | `handler_rules.go` |
| DELETE | `/api/v1/rules/{id}` | admin | `handler_rules.go` |
| POST | `/api/v1/rules/validate` | analyst+ | `handler_rules.go` |
| POST | `/api/v1/rules/test` | analyst+ | `handler_rules.go` |

---

## Alerts (JWT required)

| Method | Path | Role | File |
|--------|------|------|------|
| GET | `/api/v1/alerts` | viewer+ | `handler_alerts.go` |
| GET | `/api/v1/alerts/{id}` | viewer+ | `handler_alerts.go` |
| POST | `/api/v1/alerts/{id}/acknowledge` | analyst+ | `handler_alerts.go` |

---

## Incidents (JWT required)

| Method | Path | Role | File |
|--------|------|------|------|
| GET | `/api/v1/incidents` | viewer+ | `handler_incidents.go` |
| GET | `/api/v1/incidents/{id}` | viewer+ | `handler_incidents.go` |
| POST | `/api/v1/incidents/{id}/acknowledge` | analyst+ | `handler_incidents.go` |
| POST | `/api/v1/incidents/{id}/resolve` | analyst+ | `handler_incidents.go` |

---

## Auth (no JWT on login/setup/refresh)

| Method | Path | Auth | File |
|--------|------|------|------|
| POST | `/api/v1/auth/login` | none | `handler_auth.go` |
| POST | `/api/v1/auth/refresh` | cookie | `handler_auth.go` |
| POST | `/api/v1/auth/logout` | JWT | `handler_auth.go` |
| POST | `/api/v1/auth/setup` | none (first-run only) | `handler_auth.go` |

---

## Users (admin only)

| Method | Path | File |
|--------|------|------|
| GET | `/api/v1/users` | `handler_users.go` |
| POST | `/api/v1/users` | `handler_users.go` |
| GET | `/api/v1/users/{id}` | `handler_users.go` |
| PUT | `/api/v1/users/{id}` | `handler_users.go` |
| DELETE | `/api/v1/users/{id}` | `handler_users.go` |

---

## Apps (admin only)

| Method | Path | File |
|--------|------|------|
| GET | `/api/v1/apps` | `handler_stubs.go` |
| POST | `/api/v1/apps` | `handler_stubs.go` |
| GET | `/api/v1/apps/{id}` | `handler_stubs.go` |
| GET | `/api/v1/apps/{id}/key` | `handler_stubs.go` |
| POST | `/api/v1/apps/{id}/key/rotate` | `handler_stubs.go` |

---

## Audit (admin only)

| Method | Path | File |
|--------|------|------|
| GET | `/api/v1/audit` | `handler_audit.go` |

---

## System (no auth)

| Method | Path | File | Response |
|--------|------|------|----------|
| GET | `/health` | `api.go` | `{ status, components: { clickhouse } }` |
| GET | `/metrics` | chi + prometheus | Prometheus text format |
| GET | `/metrics/ingest` | `metrics/metrics.go` | Ingest-specific metrics |

---

## gRPC (port 5001)

| Service | Method | Type | File |
|---------|--------|------|------|
| `IngestService` | `StreamSignals` | client-streaming | `receiver_grpc.go` |
| `QueryService` | `GetSignals` | server-streaming | (stub) |
| `QueryService` | `GetTrace` | unary | (stub) |
| `ConfigService` | `CreateRule` / `ListRules` / etc | unary | (stub) |
| `HealthService` | `Check` | unary | (stub) |

Proto source: `proto/argus/v1/service.proto`
Generated stubs: `gen/go/argus/v1/service_grpc.pb.go`

---

## Port Map

| Port | Protocol | Purpose |
|------|---------|---------|
| 8080 | HTTP | All REST endpoints, WebSocket, health, metrics |
| 5001 | gRPC | Signal streaming, query, config |
