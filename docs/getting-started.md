# Getting Started with Argus XDR

This guide gets you from zero to a running Argus instance with real signals flowing through it in under 15 minutes.

---

## Prerequisites

| Requirement | Version | Notes |
|-------------|---------|-------|
| Go | 1.24+ | For building the binary |
| Docker + Docker Compose v2 | Any recent | For the backend stack |
| Python | 3.10+ | For the SDK and test scripts |
| `httpx` | 0.27+ | `pip install httpx` |

Optional:
- `buf` CLI (for regenerating protobuf stubs)
- Node.js 20+ (for the React dashboard)

---

## Step 1 — Start the Backend Stack

Argus requires ClickHouse, PostgreSQL, and Redis. The included `docker-compose.yml` starts all three plus the Argus server:

```bash
# Clone the repo
git clone https://github.com/argusxdr/argus.git
cd argus

# Start the full stack
docker compose up -d

# Wait for services to be healthy (~30 seconds)
docker compose ps
```

All four services should show `healthy`. If a service is stuck in `starting`, check logs:

```bash
docker compose logs argus-server
docker compose logs clickhouse
```

### Health check

```bash
curl http://localhost:8080/health
# → {"status":"healthy","clickhouse":{"status":"ok","latency_ms":54},"postgres":{"status":"ok","latency_ms":1},"redis":{"status":"ok","latency_ms":15}}
```

If any component shows `"status":"degraded"`, the server will still start, but persistence or caching may be impaired.

---

## Step 2 — Create Your First API Key

Before sending signals, you need an API key with `signals:write` scope.

### Option A — Admin setup (first run)

On first start, Argus runs a setup wizard at `http://localhost:8080`. Open it in a browser and follow the prompts to create the admin account. The setup flow will generate an initial API key.

### Option B — Via CLI

```bash
# Build the binary if you haven't already
go build -o ./argus ./cmd/argus

# Create an API key (requires a running server and admin credentials)
curl -s -X POST http://localhost:8080/api/v1/api-keys \
  -H "Authorization: Bearer <your-jwt-token>" \
  -H "Content-Type: application/json" \
  -d '{"name":"dev-key","scopes":["signals:write"]}'
```

The response includes the raw key — copy it, it's shown only once:

```json
{
  "id": "ak_01hw...",
  "name": "dev-key",
  "key": "argus_sk_...",
  "scopes": ["signals:write"]
}
```

Save this as `ARGUS_API_KEY` in your environment or `.env` file.

---

## Step 3 — Send Your First Signal

### Using curl

```bash
curl -X POST http://localhost:8080/v1/signals \
  -H "X-Argus-API-Key: argus_sk_YOUR_KEY_HERE" \
  -H "Content-Type: application/json" \
  -d '{
    "signal_id": "sig-01",
    "trace_id":  "trace-test-001",
    "layer":     4,
    "category":  "inference.latency.normal",
    "severity":  "INFO",
    "timestamp": "2026-05-18T10:00:00Z",
    "source": {
      "app_id": "my-app",
      "host":   "localhost"
    }
  }'
# → {"accepted":1,"rejected":0}
```

### Using the Python SDK

```python
import asyncio
from sdk.client import ArgusClient
from sdk.signal_builder import SignalBuilder

async def main():
    client = ArgusClient(
        base_url="http://localhost:8080",
        api_key="argus_sk_YOUR_KEY_HERE",
    )

    signal = (
        SignalBuilder()
        .layer(4)
        .category("inference.latency.normal")
        .severity("INFO")
        .trace_id("trace-test-001")
        .source(app_id="my-app", host="localhost")
        .l4_context(
            model_id="gpt-4o",
            latency_ms=320,
            prompt_tokens=128,
            completion_tokens=64,
            finish_reason="stop",
        )
        .build()
    )

    result = await client.ingest([signal])
    print(result)  # {"accepted": 1, "rejected": 0}

asyncio.run(main())
```

---

## Step 4 — Query Signals Back

```bash
curl "http://localhost:8080/v1/signals?app_id=my-app"
```

Response shape:

```json
{
  "signals": [
    {
      "signal_id": "sig-01",
      "trace_id": "trace-test-001",
      "layer": 4,
      "category": "inference.latency.normal",
      "severity": "INFO",
      "timestamp": "2026-05-18T10:00:00Z"
    }
  ],
  "next_cursor": null,
  "total_hint": 1
}
```

Signal storage is ClickHouse with a 2-second batch flush interval. If the signal doesn't appear immediately, wait a few seconds and try again.

---

## Step 5 — Open the Dashboard

The React dashboard is served at `http://localhost:3000` (requires `npm run dev` in `web/`).

```bash
cd web
npm install
npm run dev
# → http://localhost:3000
```

Log in with the admin credentials you created in Step 2. The Signals page shows a live feed of inbound signals. The Traces page shows correlated trace trees.

For the TUI alternative, see **Step 6**.

---

## Step 6 — Launch the Operator TUI (Optional)

Argus ships a bubbletea terminal UI for operators who prefer the terminal. It authenticates via JWT and connects to the same API.

```bash
# Build the binary
go build -o ./argus ./cmd/argus

# Launch — you'll be prompted to choose between TUI and web
./argus tui

# Or launch the behaviour view directly
./argus behaviour \
  --app-id my-app \
  --url http://localhost:8080 \
  --token <your-jwt-token>
```

TUI key bindings:

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate run list |
| `Enter` | Open run detail (span tree) |
| `r` | Return to list |
| `a` | Mark run as Compare A |
| `b` | Mark run as Compare B |
| `c` | Open compare view |
| `q` | Quit |
| `?` | Help overlay |

---

## Step 7 — Sending Multi-Layer Signals

Real LLM systems generate signals across multiple layers per request. Here's an example that covers a full inference trace with L1, L4, L9, and L10:

```python
from sdk.signal_builder import SignalBuilder
import uuid, time

trace_id = str(uuid.uuid4())
conv_id  = str(uuid.uuid4())

signals = [
    # L10 — user request hits the application
    SignalBuilder()
        .layer(10).category("application.session.request")
        .severity("INFO").trace_id(trace_id).conversation_id(conv_id)
        .source(app_id="my-app").build(),

    # L9 — orchestrator delegates to model
    SignalBuilder()
        .layer(9).category("orchestration.agent.delegated")
        .severity("INFO").trace_id(trace_id)
        .source(app_id="my-app").build(),

    # L4 — inference runs
    SignalBuilder()
        .layer(4).category("inference.latency.normal")
        .severity("INFO").trace_id(trace_id)
        .source(app_id="my-app")
        .l4_context(model_id="qwen2.5:1.5b", latency_ms=1200,
                    prompt_tokens=256, completion_tokens=128)
        .build(),

    # L1 — hardware metrics during inference
    SignalBuilder()
        .layer(1).category("hardware.gpu.utilization.normal")
        .severity("INFO").trace_id(trace_id)
        .source(app_id="my-app")
        .l1_context(gpu_utilization_pct=78, vram_used_bytes=4_294_967_296)
        .build(),
]

result = await client.ingest(signals)
# → {"accepted": 4, "rejected": 0}
```

All four signals share `trace_id` — the Traces view will render them as a correlated span tree.

---

## Common Issues

### `{"accepted":0,"rejected":N}` on ingest

Check the server logs:
```bash
docker compose logs argus-server | grep WARN
```

Common causes:
- Missing required fields (`signal_id`, `trace_id`, `layer`, `category`, `timestamp`)
- `timestamp` format must be RFC3339 (`2026-05-18T10:00:00Z`)
- `layer` must be an integer 1–10 (not a string like `"L4"`)
- Enum fields use proto value names: `"INTERNAL"` not `"DATA_CLASSIFICATION_INTERNAL"`

### 403 on `/api/v1/auth/login`

CSRF protection requires a double-submit cookie. Fetch the CSRF token first:

```bash
# Step 1 — get CSRF token (sets cookie, returns header)
curl -c cookies.txt -v http://localhost:8080/api/v1/auth/csrf-token 2>&1 | grep X-CSRF-Token
# X-CSRF-Token: <token>

# Step 2 — login with both cookie and header
curl -b cookies.txt -X POST http://localhost:8080/api/v1/auth/login \
  -H "X-CSRF-Token: <token>" \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@argus.local","password":"yourpassword"}'
```

### Signals not appearing in the dashboard

- Signals flush to ClickHouse every 2 seconds — wait briefly after ingesting
- Check `app_id` filter in the dashboard matches `source.app_id` in your signals
- Verify the time range filter includes your signal's timestamp

---

## Next Steps

- **Architecture:** How Argus processes signals end to end → `docs/architecture.md`
- **Signal taxonomy:** Full reference for all 11 layers → `docs/signal-taxonomy.md`
- **Configuration:** All config options, environment variables, secrets → `docs/configuration.md`
- **Contributing:** How to submit changes → `docs/contributing.md`
