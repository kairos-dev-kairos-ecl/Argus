# Notify & Alerting

> Alert dedup → upsert → incident correlation → routing → dispatch → adapter.

## Flow

```
DetectionProcessor match
  ↓
AlertRouter.WriteAlert(alert)
  ├─ Redis dedup (SET NX, 10 min)  →  skip if duplicate
  ├─ PostgreSQL upsert alerts table
  ├─ Incident correlation (≥3 alerts, same trace, 10 min window)
  └─ RoutingEngine.Evaluate(alert)
       ↓  matching routing_rules rows
       AlertDispatcher.Dispatch(alert, channelIDs)
         ↓  4-worker pool
         Notifier adapter(s)
           ├─ SlackNotifier
           ├─ PagerDutyNotifier
           ├─ EmailNotifier
           ├─ WebhookNotifier
           └─ SyslogNotifier
```

---

## AlertRouter

File: `internal/ingest/alert_router.go`

### Deduplication
- Key: `dedup:{SHA256(rule_id + app_id + layer + category)}`
- Redis SET NX EX 600 (10 min)
- If key exists → skip (same alert already firing)

### Alert Upsert
PostgreSQL `alerts` table:
```sql
INSERT INTO alerts (id, app_id, fingerprint, severity, layer, category, title, ...)
ON CONFLICT (fingerprint)
DO UPDATE SET signal_count = signal_count + 1, last_seen_at = now()
```

### Incident Correlation
If 3+ alerts share `trace_id` within 10 minutes:
```sql
INSERT INTO incidents (id, title, severity, alert_ids[], trace_ids[], status='open')
```
Updates `alerts.incident_id` for all correlated alerts.

---

## RoutingEngine

File: `internal/notify/router.go`

Loads routing rules from PostgreSQL `routing_rules` table every 5 minutes (hot-reload, no restart needed).

Rule evaluation per alert:
1. `rule.enabled == true`
2. `alert.severity >= rule.min_severity` (if set)
3. `alert.app_id == rule.app_id_filter` (if set)
4. `alert.layer == rule.layer_filter` (if set)
5. Check `suppression_rules` table — skip if alert fingerprint matches active suppression window

Returns list of matching `notification_channels` IDs.

---

## AlertDispatcher

File: `internal/notify/dispatcher.go`

Fixed worker pool (4 goroutines) with a dispatch queue.
```
Dispatch(alert, channelIDs)
  → enqueue to dispatch channel
  worker:
    → AdapterRegistry.Get(channelType)
    → CircuitBreakerAdapter.Notify(alert)
         → CircuitBreaker state check
         → Notifier.Notify(alert)
```

Circuit breaker per channel: 5 failures → open for 60s.

---

## Notifier Adapters

### SlackNotifier (`adapters/slack.go`)
- Library: `github.com/slack-go/slack`
- Sends formatted Block Kit message to configured Webhook URL
- Includes: severity badge, layer, category, signal count, trace link

### PagerDutyNotifier (`adapters/pagerduty.go`)
- Library: `github.com/PagerDuty/go-pagerduty`
- Creates/resolves PD incidents via Events API v2
- Maps Argus severity → PD severity (critical/error/warning/info)

### EmailNotifier (`adapters/email.go`)
- Pure SMTP (net/smtp)
- HTML template with alert details
- Supports TLS/STARTTLS

### WebhookNotifier (`adapters/webhook.go`)
- Generic HTTP POST
- Body: JSON-serialised `Alert` struct
- Configurable headers, timeout, retry-once

### SyslogNotifier (`adapters/syslog.go`)
- Library: `github.com/RackSec/srslog`
- Maps to RFC5424 facility/severity
- Supports UDP/TCP/Unix socket

### LogAdapter (`log_adapter.go`)
- Development-only: logs alert via zap
- Always registered (fallback)

---

## Suppression

File: `internal/notify/suppression.go`

Prevents alert noise during maintenance windows or known incidents.
- Rules stored in PostgreSQL `suppression_rules` table
- Matches by fingerprint pattern + time window
- RoutingEngine checks suppression before routing

---

## Alert Lifecycle (PostgreSQL `alerts.status`)

```
open → acknowledged → resolved → closed
         ↑                ↑
     analyst ack      manual/auto
```

HTTP endpoints (from [[API Routes]]):
- `POST /api/v1/alerts/{id}/acknowledge`
- `GET  /api/v1/alerts` (filterable by status, severity, layer)

---

## File Map

| File | Component |
|------|-----------|
| `internal/ingest/alert_router.go` | AlertRouter (dedup + upsert + incident) |
| `internal/notify/router.go` | RoutingEngine (PG routing rules) |
| `internal/notify/dispatcher.go` | AlertDispatcher (worker pool) |
| `internal/notify/adapter.go` | Notifier interface + AdapterRegistry |
| `internal/notify/suppression.go` | SuppressionEngine |
| `internal/notify/circuitbreaker.go` | CircuitBreaker (generic) |
| `internal/notify/circuitbreaker_adapter.go` | CB wrapping Notifier |
| `internal/notify/log_adapter.go` | LogAdapter (dev fallback) |
| `internal/notify/adapters/slack.go` | Slack notifications |
| `internal/notify/adapters/pagerduty.go` | PagerDuty notifications |
| `internal/notify/adapters/email.go` | Email (SMTP) notifications |
| `internal/notify/adapters/webhook.go` | Generic HTTP webhook |
| `internal/notify/adapters/syslog.go` | Syslog notifications |
| `internal/alert/models.go` | Alert + Incident structs |
| `internal/alert/service.go` | PostgresAlertService CRUD |
| `internal/alert/incident.go` | IncidentService |
| `internal/alert/dedup.go` | Fingerprint helpers |
