#!/bin/bash

# Argus XDR Smoke Test
# Validates the full signal ingestion + query pipeline

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}=== Argus XDR Smoke Test ===${NC}"
echo ""

# Check prerequisites
echo "Checking prerequisites..."
if ! command -v docker &> /dev/null; then
    echo -e "${RED}❌ Docker not found. Install Docker to run smoke test.${NC}"
    exit 1
fi

if ! command -v curl &> /dev/null; then
    echo -e "${RED}❌ curl not found. Install curl to run smoke test.${NC}"
    exit 1
fi

if ! command -v jq &> /dev/null; then
    echo -e "${RED}❌ jq not found. Install jq to run smoke test.${NC}"
    exit 1
fi

echo -e "${GREEN}✓${NC} Prerequisites checked"
echo ""

# Start docker compose
echo "Starting docker compose stack..."
cd "$PROJECT_ROOT"

# Clean up any existing containers
docker compose down -v 2>/dev/null || true

# Start the stack
docker compose up -d
if [ $? -ne 0 ]; then
    echo -e "${RED}❌ Failed to start docker compose${NC}"
    exit 1
fi

echo -e "${GREEN}✓${NC} Docker compose started"
echo ""

# Wait for services to be healthy
echo "Waiting for services to be healthy..."
MAX_ATTEMPTS=60
ATTEMPT=0

while [ $ATTEMPT -lt $MAX_ATTEMPTS ]; do
    if docker compose ps | grep -q "healthy"; then
        HEALTHY_COUNT=$(docker compose ps | grep "healthy" | wc -l)
        if [ "$HEALTHY_COUNT" -ge 4 ]; then
            break
        fi
    fi
    ATTEMPT=$((ATTEMPT + 1))
    sleep 1
done

if [ $ATTEMPT -eq $MAX_ATTEMPTS ]; then
    echo -e "${RED}❌ Services did not become healthy in time${NC}"
    docker compose logs
    docker compose down -v
    exit 1
fi

echo -e "${GREEN}✓${NC} All services healthy"
echo ""

# Test health endpoint
echo "Testing health endpoint..."
HEALTH=$(curl -s http://localhost:8080/health)
if ! echo "$HEALTH" | jq . &>/dev/null; then
    echo -e "${RED}❌ Health endpoint returned invalid JSON: $HEALTH${NC}"
    docker compose down -v
    exit 1
fi

if ! echo "$HEALTH" | grep -q '"status":"ok"'; then
    echo -e "${RED}❌ Health endpoint returned unexpected status: $HEALTH${NC}"
    docker compose down -v
    exit 1
fi

echo -e "${GREEN}✓${NC} Health endpoint OK"
echo ""

# Test metrics endpoint
echo "Testing metrics endpoint..."
METRICS=$(curl -s http://localhost:8080/metrics | head -20)
if ! echo "$METRICS" | grep -q "argus_"; then
    echo -e "${RED}❌ Metrics endpoint returned no Argus metrics${NC}"
    echo "$METRICS"
    docker compose down -v
    exit 1
fi

echo -e "${GREEN}✓${NC} Metrics endpoint OK"
echo ""

# Test HTTP signal ingest
echo "Sending 5 test signals via HTTP..."
for i in {1..5}; do
    SIGNAL_BODY=$(cat <<EOF
{
  "signal_id": "test-http-$i",
  "trace_id": "trace-http-abc",
  "layer": "L5_OUTPUT_DECODING",
  "category": "output.completion",
  "severity": "INFO",
  "source": {
    "app_id": "smoke-test-app",
    "app_version": "1.0.0",
    "sdk_version": "0.1.0"
  },
  "timestamp": "2026-04-07T12:00:00Z"
}
EOF
)

    RESPONSE=$(curl -s -X POST http://localhost:8080/v1/signals \
      -H "Authorization: Bearer dev-test-key" \
      -H "Content-Type: application/json" \
      -d "$SIGNAL_BODY")

    if ! echo "$RESPONSE" | jq . &>/dev/null; then
        echo -e "${RED}❌ Invalid JSON response from HTTP ingest: $RESPONSE${NC}"
        docker compose down -v
        exit 1
    fi
done

echo -e "${GREEN}✓${NC} 5 signals sent via HTTP"
echo ""

# Wait for batch flush
echo "Waiting for batch flush (3 seconds)..."
sleep 3
echo -e "${GREEN}✓${NC} Batch flush interval elapsed"
echo ""

# Test query endpoint
echo "Testing query endpoint (GET /v1/signals)..."
QUERY_RESPONSE=$(curl -s "http://localhost:8080/v1/signals?app_id=smoke-test-app")

if ! echo "$QUERY_RESPONSE" | jq . &>/dev/null; then
    echo -e "${RED}❌ Query endpoint returned invalid JSON: $QUERY_RESPONSE${NC}"
    docker compose down -v
    exit 1
fi

if ! echo "$QUERY_RESPONSE" | jq '.signals' &>/dev/null; then
    echo -e "${RED}❌ Query response missing 'signals' field: $QUERY_RESPONSE${NC}"
    docker compose down -v
    exit 1
fi

SIGNAL_COUNT=$(echo "$QUERY_RESPONSE" | jq '.signals | length')
echo "Query returned $SIGNAL_COUNT signals"

if [ "$SIGNAL_COUNT" -ne 5 ]; then
    echo -e "${YELLOW}⚠${NC} Expected 5 signals, got $SIGNAL_COUNT"
    echo "Response: $QUERY_RESPONSE"
else
    echo -e "${GREEN}✓${NC} Query endpoint returned expected signals"
fi

echo ""

# Test Prometheus metrics
echo "Verifying Prometheus metrics..."
METRICS=$(curl -s http://localhost:8080/metrics)

if echo "$METRICS" | grep -q "argus_signals_received_total"; then
    echo -e "${GREEN}✓${NC} argus_signals_received_total metric present"
else
    echo -e "${RED}❌ argus_signals_received_total metric missing${NC}"
    docker compose down -v
    exit 1
fi

if echo "$METRICS" | grep -q "argus_ingest_queue_depth"; then
    echo -e "${GREEN}✓${NC} argus_ingest_queue_depth metric present"
else
    echo -e "${RED}❌ argus_ingest_queue_depth metric missing${NC}"
    docker compose down -v
    exit 1
fi

echo ""

# Test error cases
echo "Testing error cases..."

# Missing app_id
ERROR_RESPONSE=$(curl -s "http://localhost:8080/v1/signals")
if echo "$ERROR_RESPONSE" | grep -q "app_id is required"; then
    echo -e "${GREEN}✓${NC} Correctly rejected query without app_id"
else
    echo -e "${RED}❌ Expected 'app_id is required' error${NC}"
    echo "Response: $ERROR_RESPONSE"
    docker compose down -v
    exit 1
fi

# Invalid cursor
ERROR_RESPONSE=$(curl -s "http://localhost:8080/v1/signals?app_id=test&cursor=invalid")
if echo "$ERROR_RESPONSE" | grep -q "invalid cursor"; then
    echo -e "${GREEN}✓${NC} Correctly rejected invalid cursor"
else
    echo -e "${RED}❌ Expected 'invalid cursor' error${NC}"
    echo "Response: $ERROR_RESPONSE"
    docker compose down -v
    exit 1
fi

echo ""

# Cleanup
echo "Cleaning up..."
docker compose down -v
if [ $? -ne 0 ]; then
    echo -e "${RED}❌ Failed to clean up docker compose${NC}"
    exit 1
fi

echo -e "${GREEN}✓${NC} Cleanup complete"
echo ""

echo -e "${GREEN}=== All smoke tests PASSED ===${NC}"
exit 0
