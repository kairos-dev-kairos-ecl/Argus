#!/bin/bash
set -e

# Argus XDR Installation Script
# Usage: curl -fsSL https://raw.githubusercontent.com/kairos-dev-kairos-ecl/Argus/main/install.sh | bash
# ENV: ARGUS_VERSION, ARGUS_DATA_DIR, ARGUS_PORT, NO_BROWSER

ARGUS_VERSION="${ARGUS_VERSION:-latest}"
ARGUS_DATA_DIR="${ARGUS_DATA_DIR:-.argus}"
ARGUS_PORT="${ARGUS_PORT:-8080}"
ARGUS_GRPC_PORT="${ARGUS_GRPC_PORT:-5001}"
NO_BROWSER="${NO_BROWSER:-}"
GITHUB_REPO="kairos-dev-kairos-ecl/Argus"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Utility functions
log() { echo -e "${BLUE}→${NC} $*"; }
success() { echo -e "${GREEN}✓${NC} $*"; }
warn() { echo -e "${YELLOW}⚠${NC} $*"; }
error() { echo -e "${RED}✗${NC} $*"; exit 1; }

log "Argus XDR Installation"
echo ""

# ============================================================================
# 1. OS & Architecture Detection
# ============================================================================
OS=$(uname -s)
ARCH=$(uname -m)

case "$OS" in
  Darwin) OS_TYPE="darwin" ;;
  Linux) OS_TYPE="linux" ;;
  *)
    error "Unsupported OS: $OS (only Linux and macOS supported)"
esac

case "$ARCH" in
  x86_64) ARCH_TYPE="amd64" ;;
  arm64|aarch64) ARCH_TYPE="arm64" ;;
  *)
    error "Unsupported architecture: $ARCH (only amd64 and arm64 supported)"
esac

success "Detected ${OS_TYPE}/${ARCH_TYPE}"

# ============================================================================
# 2. Container Runtime Check (Docker or Podman)
# ============================================================================
if command -v docker &> /dev/null; then
  CONTAINER_CMD="docker"
  success "Found Docker"
elif command -v podman &> /dev/null; then
  CONTAINER_CMD="podman"
  success "Found Podman"
else
  error "Docker or Podman not found. Install from https://docs.docker.com/get-docker/"
fi

# Verify container runtime is working
if ! $CONTAINER_CMD version &> /dev/null; then
  error "Container runtime failed — check Docker/Podman daemon is running"
fi

# ============================================================================
# 3. Directory Setup
# ============================================================================
mkdir -p "$ARGUS_DATA_DIR/data"
mkdir -p "$ARGUS_DATA_DIR/.secrets"
success "Created directories: $ARGUS_DATA_DIR"

# ============================================================================
# 4. Download & Verify Binary
# ============================================================================
log "Downloading Argus binary (${OS_TYPE}-${ARCH_TYPE})..."

if [ "$ARGUS_VERSION" = "latest" ]; then
  # Fetch latest release tag from GitHub API
  LATEST_TAG=$(curl -fsSL https://api.github.com/repos/$GITHUB_REPO/releases/latest | grep '"tag_name"' | head -1 | cut -d'"' -f4)
  if [ -z "$LATEST_TAG" ]; then
    error "Failed to fetch latest release from GitHub"
  fi
  ARGUS_VERSION="$LATEST_TAG"
fi

BINARY_NAME="argus-${OS_TYPE}-${ARCH_TYPE}"
BINARY_URL="https://github.com/$GITHUB_REPO/releases/download/${ARGUS_VERSION}/${BINARY_NAME}"
CHECKSUMS_URL="https://github.com/$GITHUB_REPO/releases/download/${ARGUS_VERSION}/checksums.txt"

BINARY_PATH="$ARGUS_DATA_DIR/bin/$BINARY_NAME"
mkdir -p "$ARGUS_DATA_DIR/bin"

# Download binary with retry logic
MAX_RETRIES=3
RETRY_COUNT=0
while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
  if curl -fsSL -o "$BINARY_PATH" "$BINARY_URL"; then
    break
  fi
  RETRY_COUNT=$((RETRY_COUNT + 1))
  if [ $RETRY_COUNT -ge $MAX_RETRIES ]; then
    error "Failed to download binary after $MAX_RETRIES attempts"
  fi
  warn "Download failed, retrying... ($RETRY_COUNT/$MAX_RETRIES)"
  sleep 2
done

# Download and verify checksum
if curl -fsSL -o "$ARGUS_DATA_DIR/.checksums.txt" "$CHECKSUMS_URL"; then
  # Verify checksum
  cd "$ARGUS_DATA_DIR/bin"
  if command -v sha256sum &> /dev/null; then
    if ! grep "$BINARY_NAME" "$ARGUS_DATA_DIR/.checksums.txt" | sha256sum -c - &> /dev/null; then
      error "Checksum verification failed"
    fi
  elif command -v shasum &> /dev/null; then
    if ! grep "$BINARY_NAME" "$ARGUS_DATA_DIR/.checksums.txt" | shasum -a 256 -c - &> /dev/null; then
      error "Checksum verification failed"
    fi
  else
    warn "sha256sum not found, skipping checksum verification"
  fi
  cd - > /dev/null
else
  warn "Failed to download checksums, skipping verification"
fi

# Make binary executable
chmod +x "$BINARY_PATH"
success "Binary downloaded and verified: $BINARY_PATH"

# ============================================================================
# 5. Install Binary to System PATH
# ============================================================================
if [ -w /usr/local/bin ]; then
  cp "$BINARY_PATH" /usr/local/bin/argus
  success "Installed argus to /usr/local/bin/argus"
elif [ -w "$HOME/.local/bin" ]; then
  mkdir -p "$HOME/.local/bin"
  cp "$BINARY_PATH" "$HOME/.local/bin/argus"
  if ! echo "$PATH" | grep -q "$HOME/.local/bin"; then
    warn "Add $HOME/.local/bin to PATH: export PATH=\"\$HOME/.local/bin:\$PATH\""
  fi
  success "Installed argus to $HOME/.local/bin/argus"
else
  warn "Cannot write to /usr/local/bin or $HOME/.local/bin"
  warn "Using binary from: $BINARY_PATH"
fi

# ============================================================================
# 6. Generate Configuration
# ============================================================================
log "Generating configuration..."

# Generate secure PostgreSQL password if not exists
if [ ! -f "$ARGUS_DATA_DIR/.secrets/pg_password" ]; then
  PG_PASSWORD=$(openssl rand -base64 12)
  echo "$PG_PASSWORD" > "$ARGUS_DATA_DIR/.secrets/pg_password"
  chmod 600 "$ARGUS_DATA_DIR/.secrets/pg_password"
else
  PG_PASSWORD=$(cat "$ARGUS_DATA_DIR/.secrets/pg_password")
fi

# Create main configuration file
cat > "$ARGUS_DATA_DIR/argus.yaml" << EOF
# Argus XDR Configuration
# Generated at $(date)

server:
  http:
    addr: "0.0.0.0:${ARGUS_PORT}"
  grpc:
    addr: "0.0.0.0:${ARGUS_GRPC_PORT}"

database:
  postgres:
    dsn: "postgres://argus:${PG_PASSWORD}@localhost:5432/argus?sslmode=disable"
  clickhouse:
    dsn: "clickhouse://localhost:9000/default"
  redis:
    dsn: "redis://localhost:6379/0"

storage:
  data_dir: "$ARGUS_DATA_DIR/data"
  retention:
    signals: "30d"
    incidents: "90d"
    audit: "365d"

logging:
  level: "info"
  format: "json"

ingest:
  queue:
    capacity: 100000
  batch:
    size: 500
    interval: "2s"
EOF

success "Configuration created: $ARGUS_DATA_DIR/argus.yaml"

# ============================================================================
# 7. Download docker-compose.yml from GitHub (if not exists locally)
# ============================================================================
log "Setting up Docker Compose environment..."

COMPOSE_FILE="$ARGUS_DATA_DIR/docker-compose.yml"
if [ ! -f "$COMPOSE_FILE" ]; then
  COMPOSE_URL="https://raw.githubusercontent.com/$GITHUB_REPO/${ARGUS_VERSION}/docker-compose.yml"
  if ! curl -fsSL -o "$COMPOSE_FILE" "$COMPOSE_URL"; then
    warn "Failed to download docker-compose.yml from GitHub"
    warn "Using minimal fallback configuration"
    # Fallback: minimal compose config
    cat > "$COMPOSE_FILE" << 'COMPOSE_EOF'
version: "3.9"
services:
  argus:
    image: ghcr.io/kairos-dev-kairos-ecl/argus:latest
    ports:
      - "8080:8080"
      - "5001:5001"
    environment:
      ARGUS_POSTGRES_DSN: "postgres://argus:password@postgres:5432/argus"
      ARGUS_CLICKHOUSE_DSN: "clickhouse://clickhouse:9000/default"
      ARGUS_REDIS_DSN: "redis://redis:6379/0"
    depends_on:
      - postgres
      - clickhouse
      - redis
  clickhouse:
    image: clickhouse/clickhouse-server:24-alpine
    ports:
      - "8123:8123"
    volumes:
      - clickhouse_data:/var/lib/clickhouse
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: argus
      POSTGRES_PASSWORD: password
      POSTGRES_DB: argus
    volumes:
      - postgres_data:/var/lib/postgresql/data
  redis:
    image: redis:7.2-alpine
    ports:
      - "6379:6379"
volumes:
  clickhouse_data:
  postgres_data:
COMPOSE_EOF
  fi
fi

success "Docker Compose configured: $COMPOSE_FILE"

# ============================================================================
# 8. Start Services
# ============================================================================
log "Starting services via docker-compose..."

cd "$ARGUS_DATA_DIR"
POSTGRES_PASSWORD="$PG_PASSWORD" $CONTAINER_CMD compose up -d --wait 2>&1 | grep -E "Started|Created|done" || true

# Wait for services to be healthy
sleep 3

# Check service health
HEALTH_CHECK_PASSED=true
if ! curl -sf http://localhost:9090/health &> /dev/null; then
  warn "Argus API health check failed (still starting)"
  HEALTH_CHECK_PASSED=false
fi

success "Services starting (logs available via: $CONTAINER_CMD compose logs -f)"

# ============================================================================
# 9. Print Summary
# ============================================================================
echo ""
echo "╔════════════════════════════════════════════════╗"
echo "║  Argus XDR Installation Complete              ║"
echo "╚════════════════════════════════════════════════╝"
echo ""
echo "Dashboard:   ${GREEN}http://localhost:${ARGUS_PORT}/setup${NC}"
echo "API:         ${GREEN}http://localhost:${ARGUS_PORT}/api${NC}"
echo "gRPC:        ${BLUE}localhost:${ARGUS_GRPC_PORT}${NC}"
echo ""
echo "Configuration:"
echo "  ${BLUE}$ARGUS_DATA_DIR/argus.yaml${NC}"
echo ""
echo "Commands:"
echo "  Start:   ${YELLOW}argus server start${NC}"
echo "  Stop:    ${YELLOW}argus server stop${NC}"
echo "  Status:  ${YELLOW}argus server status${NC}"
echo "  Logs:    ${YELLOW}argus server logs${NC}"
echo "  Doctor:  ${YELLOW}argus doctor${NC}"
echo ""
echo "Container logs:"
echo "  ${YELLOW}cd $ARGUS_DATA_DIR && docker-compose logs -f${NC}"
echo ""

# ============================================================================
# 10. Open Browser
# ============================================================================
if [[ -z "$NO_BROWSER" ]]; then
  log "Opening browser..."
  sleep 2
  if [[ "$OS_TYPE" == "darwin" ]]; then
    open "http://localhost:${ARGUS_PORT}/setup" || true
  else
    xdg-open "http://localhost:${ARGUS_PORT}/setup" 2>/dev/null || \
    which x-www-browser > /dev/null && x-www-browser "http://localhost:${ARGUS_PORT}/setup" || \
    echo "Open your browser to: http://localhost:${ARGUS_PORT}/setup"
  fi
fi

success "Installation complete!"
