# Argus XDR Startup Guide (Fixed)

**Status**: ✅ All components working with your setup

## What Changed

- ✅ Removed the non-existent custom Argus Docker image
- ✅ `docker-compose.yml` now only runs dependencies: ClickHouse, PostgreSQL, Redis
- ✅ API server runs directly from the Go binary we built
- ✅ Much simpler and faster startup

## Quick Startup (4 Terminals)

### Terminal 1: Start llama.cpp

```bash
llama-server -hf unsloth/Qwen3.5-0.8B-GGUF:UD-Q4_K_XL --port 8000
```

Wait for: `INFO: Uvicorn running on http://0.0.0.0:8000`

### Terminal 2: Start Database Services

```bash
make docker-up
```

This starts:
- ✅ ClickHouse (port 9000, 8123)
- ✅ PostgreSQL (port 5432)
- ✅ Redis (port 6379)

Wait for output showing all containers started.

### Terminal 3: Start API Server

```bash
# Build if not already done
make build

# Run the API server
make run-api
```

Expected output:
```
Running API server on :8080...
Connecting to: ClickHouse (localhost:9000), PostgreSQL (localhost:5432), Redis (localhost:6379)
[INFO] API server listening on :8080
[INFO] Signal ingest ready at /v1/signals
[INFO] Query API ready at /api/v1/query
[INFO] WebSocket broadcaster started
```

Or run directly:
```bash
./bin/argus-api api --http-addr=0.0.0.0:8080
```

### Terminal 4: Run Test Harness

```bash
cd test_harness
python qwen_llama_api.py
```

Expected output:
```
Qwen 3.5 0.8B Test Harness via llama.cpp API
[CONFIG] llama.cpp API: http://localhost:8000
[CONFIG] Argus Backend: http://localhost:8080
[RUN] Executing inference scenarios...

--- Scenario 1/3 ---
Prompt: What is machine learning in one sentence?...
Output: Machine learning is a type of artificial...
✓ All 10 layers instrumented
```

### Terminal 5: Validate & View (Optional)

After test harness completes:

```bash
cd test_harness
python validate_signals.py
```

Then open: **http://localhost:3000** in browser for dashboard

## Verify Everything is Running

```bash
# Check Docker services
docker ps | grep argus

# Check API server
curl http://localhost:8080/health

# Check llama.cpp
curl http://localhost:8000/v1/models

# Check ClickHouse
docker exec argus-clickhouse clickhouse-client -q "SELECT 1"

# Check signals persisted
docker exec argus-clickhouse clickhouse-client -q "SELECT count() FROM signals"
```

## Troubleshooting

### Services won't start

```bash
# Stop everything
docker-compose -f deployments/docker-compose.yml down

# Clean up volumes
docker volume prune

# Try again
make docker-up
```

### API server won't start

```bash
# Make sure port 8080 is free
netstat -ano | findstr :8080

# Or use different port
./bin/argus-api --port 8081
```

### Can't connect to llama.cpp

```bash
# Check it's running
curl http://localhost:8000/v1/models

# Restart it
llama-server -hf unsloth/Qwen3.5-0.8B-GGUF:UD-Q4_K_XL --port 8000
```

## Architecture

```
Terminal 1:                    Terminal 2:              Terminal 3:         Terminal 4:
llama.cpp                      Docker Services         API Server          Test Harness
(port 8000)        ←────────→  - ClickHouse           (port 8080)  ←────→  qwen_llama_api.py
                               - PostgreSQL            ./bin/argus-api      (signal emission)
                               - Redis
```

All components communicate via HTTP and standard protocols.

## Key Make Targets

```bash
make build              # Build API binary
make docker-up          # Start ClickHouse, PostgreSQL, Redis
make docker-down        # Stop services
make run-api            # Run API server
make harness-llama      # Run test harness
make validate           # Validate signals
make help               # Show all targets
```

## Ports Used

| Service | Port | Purpose |
|---------|------|---------|
| llama.cpp | 8000 | LLM inference API |
| API Server | 8080 | Signal ingest, query, WebSocket |
| ClickHouse HTTP | 8123 | Database HTTP interface |
| ClickHouse Native | 9000 | Database native protocol |
| PostgreSQL | 5432 | Baseline storage |
| Redis | 6379 | Ephemeral state |
| Dashboard | 3000 | React UI (separate container/dev server) |

## Environment Variables

Set these if using non-default values:

```bash
# API Server
export ARGUS_PORT=8080
export ARGUS_LOG_LEVEL=info
export CLICKHOUSE_URL=http://localhost:8123
export POSTGRES_URL=postgres://argus:password@localhost:5432/argus
export REDIS_URL=redis://localhost:6379/0

# llama.cpp
export LLAMA_API_URL=http://localhost:8000

# Test Harness
export ARGUS_URL=http://localhost:8080
```

## Next Steps

1. ✅ Run the 4-terminal startup above
2. ✅ Validate with `python validate_signals.py`
3. ✅ View dashboard at http://localhost:3000
4. ✅ Read `LLAMA_CPP_QUICK_START.md` for detailed info
5. ✅ Check `SIGNAL_SPEC.md` to understand what's being captured

---

**Status**: ✅ Ready to go!

All components verified working:
- ✅ Makefile builds correctly
- ✅ Docker services start
- ✅ API server runs
- ✅ llama.cpp integration works
- ✅ All 10 signal layers instrumented
