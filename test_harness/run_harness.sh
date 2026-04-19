#!/usr/bin/env bash
# run_harness.sh — One-command Argus backend validation harness
#
# Usage: bash test_harness/run_harness.sh
#
# What it does:
#   1. Verifies the Qwen gguf model exists in test_harness/models/
#   2. Starts all 5 Docker services (docker-compose-test.yml)
#   3. Waits up to 120s for all services to be healthy
#   4. Runs instrumented_llm.py to drive prompts and emit signals
#   5. Sleeps 10s to let the batch writer flush (batch interval = 2s)
#   6. Runs validate_capture.py to produce the coverage report
#   7. Prints final status and points at validation_results.md

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${REPO_ROOT}/docker-compose-test.yml"
MODELS_DIR="${SCRIPT_DIR}/models"
MODEL_FILE="${MODELS_DIR}/qwen2.5-0.5b-instruct-q4_k_m.gguf"
RESULTS_FILE="${SCRIPT_DIR}/validation_results.md"
MAX_WAIT=120

# ANSI colours (safe to use — CI typically has TERM set)
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Colour

log()  { echo -e "${GREEN}[harness]${NC} $*"; }
warn() { echo -e "${YELLOW}[harness]${NC} $*"; }
err()  { echo -e "${RED}[harness]${NC} $*" >&2; }

# ---------------------------------------------------------------------------
# Step 1: Verify model file
# ---------------------------------------------------------------------------
log "Checking for model file…"
if [[ ! -f "${MODEL_FILE}" ]]; then
    err "Model file not found: ${MODEL_FILE}"
    err ""
    err "Download it first:"
    err "  pip install huggingface_hub[cli]"
    err "  huggingface-cli download Qwen/Qwen2.5-0.5B-Instruct-GGUF \\"
    err "    qwen2.5-0.5b-instruct-q4_k_m.gguf \\"
    err "    --local-dir test_harness/models/"
    err ""
    err "See test_harness/README.md for full instructions."
    exit 1
fi
log "Model found: ${MODEL_FILE}"

# ---------------------------------------------------------------------------
# Step 2: Start Docker stack
# ---------------------------------------------------------------------------
log "Starting test Docker stack…"
docker compose -f "${COMPOSE_FILE}" up -d

# ---------------------------------------------------------------------------
# Step 3: Wait for all services to be healthy (max 120s)
# ---------------------------------------------------------------------------
log "Waiting for services to become healthy (max ${MAX_WAIT}s)…"

# Services we expect to be healthy
SERVICES=(
    "argus-clickhouse-test"
    "argus-postgres-test"
    "argus-redis-test"
    "argus-server-test"
    "argus-llamacpp-test"
)

elapsed=0
while true; do
    all_healthy=true
    for svc in "${SERVICES[@]}"; do
        health=$(docker inspect --format='{{.State.Health.Status}}' "${svc}" 2>/dev/null || echo "absent")
        if [[ "${health}" != "healthy" ]]; then
            all_healthy=false
            break
        fi
    done

    if ${all_healthy}; then
        log "All 5 services are healthy."
        break
    fi

    if (( elapsed >= MAX_WAIT )); then
        err "Timed out waiting for services after ${MAX_WAIT}s."
        err "Current status:"
        docker compose -f "${COMPOSE_FILE}" ps
        exit 1
    fi

    sleep 5
    elapsed=$(( elapsed + 5 ))
    warn "  Still waiting… (${elapsed}s / ${MAX_WAIT}s)"
done

# ---------------------------------------------------------------------------
# Step 4: Run instrumented LLM client
# ---------------------------------------------------------------------------
log "Running instrumented_llm.py…"
ARGUS_URL="http://localhost:8082" \
LLAMA_URL="http://localhost:8081" \
python3 "${SCRIPT_DIR}/instrumented_llm.py"

# ---------------------------------------------------------------------------
# Step 5: Sleep to let batch writer flush
# ---------------------------------------------------------------------------
log "Waiting 10s for batch writer to flush…"
sleep 10

# ---------------------------------------------------------------------------
# Step 6: Run ClickHouse validator
# ---------------------------------------------------------------------------
log "Running validate_capture.py…"
CH_HOST=localhost \
CH_PORT=8124 \
python3 "${SCRIPT_DIR}/validate_capture.py"

# ---------------------------------------------------------------------------
# Step 7: Final status
# ---------------------------------------------------------------------------
if [[ -f "${RESULTS_FILE}" ]]; then
    log "Validation results written to: ${RESULTS_FILE}"
else
    warn "validation_results.md not found — check validator output above."
fi

log ""
log "Harness complete. To tear down the test stack, run:"
log "  bash test_harness/teardown.sh"
