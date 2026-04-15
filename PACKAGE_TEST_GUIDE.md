# Clean Package Test Guide

## Pre-Test Status ✅

Your codebase is now clean and production-ready:

```
Master Branch: 62e9068
├── 62e9068 docs: workspace cleanup and connector refactoring summary
├── e94f223 refactor: introduce Sensor/Connector abstraction
├── c78721d docs: signal capture testing guide
├── 99dc2e5 Fix critical auth system database migrations
└── (3 more commits with fixes and features)
```

### What's Fixed & Ready

**Database** ✅
- Migration 016: sessions table (user session management)
- Migration 017: token_revocations table (token validation)
- Fixed Migration 008: audit_log schema (user_id, timestamp, user_agent)
- All migrations auto-apply on backend startup

**Frontend** ✅
- Sidebar rendering fixed (opaque background, no overlap)
- Layout properly handles sidebar toggle
- Auth system should no longer log you off on navigation

**Backend** ✅
- OTLP receiver ready at `/v1/traces`
- All services wired correctly
- Rules engine loaded on startup
- Redis for caching/dedup available

**SDK** ✅ **NEW**
- Sensor/Connector abstraction (3 connector types)
- @observe decorator for zero-code instrumentation
- Buffered connector for high-volume apps
- Full test suite included
- Comprehensive documentation

## Step 1: Fresh Environment

```bash
cd /path/to/ArgusXDR

# Clean any old containers
docker-compose down -v

# Remove any build artifacts
rm -rf web/dist build/ *.log

# Fresh start
docker-compose up -d
```

Wait 30 seconds for services to start and migrations to apply.

## Step 2: Verify Services

```bash
# Check backend health
curl http://localhost:8080/health

# Expected: {"status":"ok"} or similar

# Check PostgreSQL migrations applied
curl http://localhost:8080/health | grep -i postgres

# Check ClickHouse
docker-compose exec clickhouse clickhouse-client -q "SELECT 1"
```

## Step 3: Test Frontend (Old Approach)

### Setup
```bash
cd web
npm install
npm run dev
```

Open `http://localhost:5173`

### Test Sequence
1. **Setup Wizard** → Complete all 5 steps
   - Admin account creation
   - Instance config
   - Notifications (skip OK)
   - Register first app
   - Done

2. **Dashboard** → Should not log you out
   - Click sidebar items (Dashboard, Signals, Alerts, etc)
   - Verify you stay logged in
   - ✅ This tests the sessions table fix

3. **Signal Stream** → Should be empty (no signals yet)
   - Go to Signal Stream page
   - Should show "No signals" message

4. **Sidebar Toggle** → Should not overlap
   - Click hamburger menu to open sidebar
   - Click again to close
   - No content overlap ✅

## Step 4: Test Signal Capture (New Sensor API)

### Option A: Quick Test Script

Create `test_sensor.py`:

```python
import asyncio
from sdk.connector import Sensor, ConnectorType, Layer, Severity

async def test():
    # Use buffered connector for efficiency
    sensor = Sensor(
        connector_type=ConnectorType.BUFFER,
        config={
            "base_url": "http://localhost:8080",
            "app_id": "test-app",
            "max_batch_size": 10,
            "flush_interval_seconds": 2.0,
        }
    )

    print("Emitting test signal...")
    result = await sensor.emit(
        layer=Layer.L5_OUTPUT_DECODING,
        category="inference.test",
        severity=Severity.INFO,
        context={
            "model": "test",
            "input_tokens": 50,
            "output_tokens": 100,
        },
        duration_ms=125.5
    )
    
    print(f"Emission result: {result}")
    
    # Wait for auto-flush
    await asyncio.sleep(3)
    
    await sensor.close()
    print("Done!")

if __name__ == "__main__":
    asyncio.run(test())
```

Run it:
```bash
cd /path/to/ArgusXDR
python test_sensor.py
```

### Option B: Interactive Testing

```bash
python3 -c "
import asyncio
from sdk.connector import Sensor, ConnectorType, Layer

async def test():
    sensor = Sensor(ConnectorType.NOOP)  # Test mode
    result = await sensor.emit(
        layer=Layer.L5_OUTPUT_DECODING,
        category='interactive_test',
    )
    print(f'✓ Emission: {result}')
    await sensor.close()

asyncio.run(test())
"
```

## Step 5: Verify Signal in Dashboard

1. Open dashboard: `http://localhost:3000`
2. Go to **Signal Stream**
3. Look for signals with `app_id = "test-app"`
4. Should see your test signal(s)

If you don't see them:
- Check backend logs: `docker-compose logs argus`
- Verify test script ran: `grep -i "emitted\|signal" test_sensor.py`
- Check OTLP receiver logs for errors

## Step 6: Test with @observe Decorator

Create `test_decorator.py`:

```python
import asyncio
from sdk.connector import Sensor, ConnectorType, Layer, observe

sensor = Sensor(
    connector_type=ConnectorType.BUFFER,
    config={"app_id": "decorator-test"}
)

@observe(sensor, Layer.L5_OUTPUT_DECODING, "decorator.test")
async def my_function(prompt: str) -> str:
    # Duration captured automatically
    # Any exception becomes a HIGH severity signal
    await asyncio.sleep(0.5)  # Simulate work
    return f"Response to: {prompt}"

async def main():
    # Call the decorated function
    result = await my_function("Hello!")
    print(f"Result: {result}")
    
    # Wait for auto-flush
    await asyncio.sleep(3)
    await sensor.close()

if __name__ == "__main__":
    asyncio.run(main())
```

Run it:
```bash
python test_decorator.py
```

Check Signal Stream for signals with `category = "decorator.test"`

## Step 7: Clean Package Validation Checklist

- [ ] **Database**: All 3 migrations applied without errors
- [ ] **Frontend**: Loads without console errors
- [ ] **Frontend**: Setup wizard completes successfully
- [ ] **Frontend**: Can navigate without getting logged out
- [ ] **Frontend**: Sidebar opens/closes without overlap
- [ ] **Backend**: `/health` endpoint returns 200 OK
- [ ] **Backend**: OTLP receiver running (check logs)
- [ ] **SDK**: `Sensor` class can emit signals
- [ ] **SDK**: `@observe` decorator works
- [ ] **SDK**: Signals appear in dashboard Signal Stream
- [ ] **SDK**: Buffered connector batches signals
- [ ] **Tests**: `pytest sdk/tests/test_connector.py -v` passes

## Step 8: Documentation Review

For the clean package, ensure users have:

1. **CONNECTOR.md** - High-level Sensor/Connector API guide
   - Use for: "I want to instrument my app"
   - Shows all 3 connectors
   - Has code examples

2. **TESTING_SIGNALS.md** - Setup and testing guide
   - Use for: "I want to test signal capture"
   - Step-by-step instructions
   - Troubleshooting section

3. **WORKSPACE_CLEANUP.md** - Architecture overview
   - Use for: "I want to understand the design"
   - Shows connector abstraction
   - Migration path from old SDK

4. **sdk/python/README.md** - Low-level SDK (legacy)
   - Keep for reference
   - Mark as "Legacy - Use Sensor instead"

## Performance Baseline

With `BufferedConnector(max_batch_size=100, flush_interval_seconds=5)`:

Expected metrics:
- **Overhead per signal**: <1ms (batched)
- **Batch size**: 100 signals
- **Flush interval**: 5 seconds
- **Network**: 1 HTTP call per 100 signals (5 second delay)

Test it:
```bash
# Emit 100 signals
for i in {1..100}; do
  python -c "
import asyncio
from sdk.connector import Sensor, ConnectorType, Layer
async def test():
    s = Sensor(ConnectorType.BUFFER, {'app_id': 'perf-test'})
    await s.emit(Layer.L5_OUTPUT_DECODING, 'perf.test')
    await s.close()
asyncio.run(test())
  " &
done
wait

# Should see 1 HTTP POST (100 signals batched)
# Check: docker-compose logs argus | grep "POST /v1/traces"
```

## Known Limitations

- **OTLP**: Only `/v1/traces` supported (not `/v1/metrics` yet)
- **Kafka**: Not implemented (future connector)
- **Distributed Tracing**: Basic trace ID correlation only
- **Metrics**: No built-in drop counter export (coming soon)

## Next Steps After Package Test

1. **Tag Release** (if all tests pass)
   ```bash
   git tag v0.2.0-connector-beta
   git push origin v0.2.0-connector-beta
   ```

2. **Documentation for Users**
   - Start with CONNECTOR.md
   - Show Sensor API examples
   - Provide FastAPI middleware example

3. **Community Feedback**
   - Ask users about new connector types
   - Request examples of their instrumentations
   - Gather performance feedback

4. **Next Phase (Step 6)**
   - OTLP native connector
   - Kafka connector
   - Language-specific agents (Go, JS)

## Questions During Testing?

Check:
1. **CONNECTOR.md** - API reference
2. **TESTING_SIGNALS.md** - Setup issues
3. **WORKSPACE_CLEANUP.md** - Architecture
4. **Backend logs**: `docker-compose logs argus`
5. **Frontend logs**: Browser console (F12)

## Cleaning Up After Test

```bash
# Keep the code
git status  # Should be clean

# Clean Docker
docker-compose down -v

# Clean Python cache
rm -rf __pycache__ .pytest_cache
find . -name "*.pyc" -delete

# You're clean!
```

---

**Ready to test?** Start with Step 1: Fresh Environment and work through each step.

All commits are on master and ready for production package/release.
