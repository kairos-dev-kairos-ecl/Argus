#!/usr/bin/env bash
# teardown.sh — Tear down the Argus test stack and remove volumes
#
# Usage: bash test_harness/teardown.sh
#
# This removes all test containers and volumes created by docker-compose-test.yml.
# The dev stack (docker-compose.yml) is NOT touched.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${REPO_ROOT}/docker-compose-test.yml"

GREEN='\033[0;32m'
NC='\033[0m'

log() { echo -e "${GREEN}[teardown]${NC} $*"; }

log "Tearing down test stack and removing volumes…"
docker compose -f "${COMPOSE_FILE}" down -v

log "Done. Test containers and volumes removed."
log ""
log "Verify with:"
log "  docker ps -a --filter name=-test"
