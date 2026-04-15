# Testing Signal Capture with Argus

## What Was Fixed

### Database Migrations (Critical)
- **Migration 016**: Created `sessions` table - required for user session management and token refresh
- **Migration 017**: Created `token_revocations` table - required for token revocation checking
- **Migration 008**: Fixed `audit_log` schema to match code expectations
  - Changed `actor_id` → `user_id` with FK to users table
  - Changed `occurred_at` → `timestamp`
  - Changed `details` → `detail` (JSONB)
  - Added `user_agent` column

These were causing:
- "ERROR: relation 'sessions' does not exist" → Frontend auth failures
- "ERROR: relation 'token_revocations' does not exist" → Token validation errors  
- "ERROR: column 'user_id' does not exist in audit_log" → Audit logging failures

### Frontend
- Fixed sidebar background color to be opaque (#1F1F23) instead of transparent
- Fixed header background for consistency
- Layout now properly handles sidebar open/close without overlap

## How to Test Signal Capture

### 1. Start the Backend
```bash
# Ensure all services are running
docker-compose up -d

# The migrations will auto-apply on startup
# Check logs: docker-compose logs argus
```

### 2. Create a Python Script to Emit Signals

Create `test_signals.py`:

```python
import asyncio
from sdk.client import ArgusClient, Layer, Severity

async def test_signal_capture():
    # Initialize client pointing to your Argus backend
    client = ArgusClient(
        base_url="http://localhost:8080",  # Adjust if backend is on different host/port
        app_id="test-app",
        app_version="1.0.0",
        environment="dev"
    )
    
    # Emit a test signal
    success = await client.emit_signal(
        layer=Layer.L5_OUTPUT_DECODING,
        category="inference.test",
        severity=Severity.INFO,
        context={
            "input_tokens": 50,
            "output_tokens": 150,
            "finish_reason": "stop",
        },
        duration_ms=125.5
    )
    
    print(f"Signal emission result: {success}")
    
    # Close client
    client.close()

if __name__ == "__main__":
    asyncio.run(test_signal_capture())
```

### 3. Run the Test Script
```bash
cd /path/to/ArgusXDR
python test_signals.py
```

### 4. Verify Signal Capture
- Open dashboard: `http://localhost:3000`
- Navigate to **Signal Stream**
- Filter by `app_id = "test-app"`
- You should see the test signal appear

## Instrumenting Your LLM Application

### Option A: Using the @observe Decorator (Recommended)

```python
from sdk.client import ArgusClient, Layer, observe

# Initialize once
argus_client = ArgusClient(
    base_url="http://localhost:8080",
    app_id="my-llm-app",
)

@observe(
    layer=Layer.L5_OUTPUT_DECODING,
    category="inference.completion",
    client=argus_client,
)
async def generate_response(prompt: str) -> str:
    # Your LLM call here (e.g., Ollama, OpenAI, etc.)
    # Duration and error status are captured automatically
    response = await call_ollama(prompt)
    return response
```

### Option B: Manual Signal Emission

```python
async with ArgusClient(base_url="http://localhost:8080") as client:
    start = time.time()
    
    # Your LLM call
    response = await call_ollama(prompt)
    
    duration_ms = (time.time() - start) * 1000
    
    await client.emit_signal(
        layer=Layer.L5_OUTPUT_DECODING,
        category="inference.ollama",
        severity=Severity.INFO,
        context={
            "model": "llama2",
            "prompt_length": len(prompt),
            "response_length": len(response),
        },
        duration_ms=duration_ms,
    )
```

## Configuration for Your Arch Setup

If testing in your Arch environment with Docker:

### Frontend (.env or Vite config)
```javascript
// vite.config.ts
export default defineConfig({
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',  // Adjust to your backend host:port
        changeOrigin: true,
        rewrite: (path) => path,
      },
    },
  },
})
```

### Backend (docker-compose.yml or .env)
```yaml
services:
  argus:
    environment:
      ARGUS_SERVER_HTTP_ADDR: "0.0.0.0:8080"
      ARGUS_DATABASE_POSTGRES_DSN: "postgres://postgres:password@postgres:5432/argus"
      ARGUS_REDIS_ADDR: "redis:6379"
      ARGUS_DATABASE_CLICKHOUSE_DSN: "clickhouse:9000"
```

### Python SDK (test script)
```python
# If backend is on different machine/network
client = ArgusClient(
    base_url="http://<your-arch-hostname>:8080",  # Use actual IP or hostname
    app_id="arch-test-app",
)
```

## Troubleshooting

### "Connection refused" on signal emission
- Verify backend is running: `curl http://localhost:8080/health`
- Check firewall/network connectivity if testing remotely
- Verify correct backend URL in client initialization

### No signals appearing in dashboard
1. Check backend logs for OTLP errors: `docker-compose logs argus | grep -i otlp`
2. Verify signal is being emitted: Add print statements in test script
3. Check that app_id matches filter in dashboard

### Auth/Session errors on frontend
- Backend migrations should have applied automatically on startup
- Check postgres logs: `docker-compose logs postgres`
- Verify all three migrations (008, 016, 017) exist in `internal/storage/migrations/`

### Sidebar still looks wrong
- Clear browser cache (Ctrl+Shift+Del or Cmd+Shift+Del)
- Hard refresh frontend (Ctrl+F5 or Cmd+Shift+R)
- Rebuild frontend: `cd web && npm run build`

## Next Steps

1. **Backend**: All database tables now exist - signal capture should work
2. **Frontend**: Layout and sidebar rendering fixed - UI should be usable
3. **Instrumentation**: Use the SDK to instrument your LLM application
4. **Dashboard**: Create detection rules in Settings to alert on signals

See `sdk/python/README.md` for full SDK documentation and examples.
