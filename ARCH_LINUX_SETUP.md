# Argus XDR — Arch Linux Setup Guide

Complete setup and testing guide for Arch Linux systems.

## Prerequisites

### System Requirements

- **OS:** Arch Linux (updated)
- **CPU:** 2+ cores recommended
- **RAM:** 4GB minimum, 8GB+ recommended
- **Disk:** 10GB free space
- **Network:** Internet connection

### Install Base Dependencies

```bash
sudo pacman -Syu
sudo pacman -S --noconfirm base-devel git curl wget
```

## Installation Options

### Option 1: Docker Compose (Recommended)

**Install Docker & Docker Compose:**

```bash
sudo pacman -S --noconfirm docker docker-compose
sudo systemctl enable docker
sudo systemctl start docker
sudo usermod -aG docker $USER
# Log out and back in for group changes to take effect
```

**Run Argus:**

```bash
git clone https://github.com/argus-xdr/argus.git
cd argus
docker-compose up -d
```

**Verify:**

```bash
docker-compose ps
# All services should show "healthy"

curl http://localhost:9090
# Should return dashboard
```

**Access Dashboard:**
- http://localhost:9090/setup (first-time setup)
- http://localhost:9090 (dashboard after setup)

### Option 2: Build from Source

**Install Go & Node.js:**

```bash
sudo pacman -S --noconfirm go nodejs npm
go version      # Should be 1.21+
node --version  # Should be 18+
npm --version   # Should be 9+
```

**Install ClickHouse, PostgreSQL, Redis:**

```bash
# Option A: Using docker (minimal)
docker run -d --name clickhouse -p 9000:9000 -p 8123:8123 clickhouse/clickhouse-server
docker run -d --name postgres -p 5432:5432 -e POSTGRES_PASSWORD=argus postgres:16
docker run -d --name redis -p 6379:6379 redis:7

# Option B: Using pacman (full installation)
sudo pacman -S --noconfirm clickhouse postgresql redis
# Then start services:
sudo systemctl start postgresql redis
# ClickHouse may need additional setup
```

**Build Backend:**

```bash
cd argus
go mod tidy
go build -o ./argus ./cmd/argus
./argus server --help
```

**Build Frontend:**

```bash
cd web
npm install
npm run build
# Or for development:
npm run dev
```

**Start Services:**

```bash
# Terminal 1: Backend
./argus server

# Terminal 2: Frontend (if using npm run dev)
cd web && npm run dev

# Terminal 3: Monitor
watch -n 1 'curl -s http://localhost:8080/metrics | grep argus_'
```

**Access Dashboard:**
- http://localhost:5173 (frontend dev server)
- http://localhost:8080 (backend API)

### Option 3: Minimal (Dev Mode)

**No external dependencies required:**

```bash
cd argus
go build -o ./argus ./cmd/argus
./argus server --dev

# In another terminal, test:
curl http://localhost:8080/health
# → {"status":"ok"}
```

## Testing on Arch Linux

### 1. Verify Installation

```bash
# Check all services are running
docker-compose ps

# Check API is responding
curl -s http://localhost:9090 | head -20

# Check metrics
curl -s http://localhost:8080/metrics | grep argus_signals
```

### 2. Run Test Harness

```bash
# Install Python dependencies
cd test_harness
pip install -r requirements.txt

# Run attack scenarios
python runner.py --scenario-set A  # Baseline
python runner.py --scenario-set B  # Prompt injection
python runner.py --scenario-set C  # Data integrity
python runner.py --scenario-set D  # Agent abuse

# Generate coverage report
python runner.py --report
```

### 3. Integrate SDKs

**Python SDK:**

```bash
pip install ./sdk/python

# In your app:
from argus import observe, Layer, Category

@observe(Layer.L7, Category.RETRIEVAL_SEARCH)
def search_documents(query):
    return documents
```

**TypeScript SDK:**

```bash
cd apps/chatbot-app
npm install
npm start
# Navigate to http://localhost:3000
```

### 4. Deploy Reference Applications

**RAG App:**

```bash
cd apps/rag-app
pip install -r requirements.txt
python app.py
# POST /ask with {"query": "your question"}
```

**Agent App:**

```bash
cd apps/agent-app
pip install -r requirements.txt
python app.py
# POST /run-agent with {"task": "your task"}
```

**Chatbot App:**

```bash
cd apps/chatbot-app
npm install
npm start
# Open http://localhost:3000
```

## Troubleshooting

### Docker Issues

```bash
# Check Docker is running
systemctl status docker

# Verify daemon socket
ls /run/docker.sock

# If permission denied, ensure user is in docker group:
groups $USER
# Should include 'docker'

# Rebuild if services fail
docker-compose down -v
docker-compose build --no-cache
docker-compose up -d
```

### Port Conflicts

```bash
# If ports are in use, check what's using them:
sudo lsof -i :9090  # Argus API
sudo lsof -i :9000  # ClickHouse
sudo lsof -i :5432  # PostgreSQL
sudo lsof -i :6379  # Redis

# Or in docker-compose.yml, change ports:
ports:
  - "9999:9090"  # Use different port
```

### Database Connection Issues

```bash
# Test ClickHouse
docker-compose exec clickhouse clickhouse-client --query "SELECT 1"

# Test PostgreSQL
docker-compose exec postgres psql -U postgres -c "SELECT 1"

# Test Redis
docker-compose exec redis redis-cli ping
# → PONG

# Check logs
docker-compose logs clickhouse
docker-compose logs postgres
docker-compose logs redis
```

### Memory Issues

If you get OOM errors, increase Docker memory:

```bash
# Edit docker-compose.yml:
services:
  clickhouse:
    deploy:
      resources:
        limits:
          memory: 4G
  postgres:
    deploy:
      resources:
        limits:
          memory: 2G
  redis:
    deploy:
      resources:
        limits:
          memory: 1G
```

### Performance Optimization

For better performance on Arch Linux:

```bash
# Increase file descriptor limits
sudo sysctl -w fs.file-max=2097152
echo "fs.file-max=2097152" | sudo tee -a /etc/sysctl.conf

# Increase connection limits
sudo sysctl -w net.core.somaxconn=65536
echo "net.core.somaxconn=65536" | sudo tee -a /etc/sysctl.conf

# Apply changes
sudo sysctl -p
```

## Next Steps

1. **Create admin account** via setup wizard (http://localhost:9090/setup)
2. **Register your app** to get API key
3. **Integrate SDK** in your LLM application
4. **Run test harness** to validate detection
5. **Deploy to production** using systemd unit or Kubernetes

See [PRODUCTION_CHECKLIST.md](docs/PRODUCTION_CHECKLIST.md) for deployment guidelines.

## Support

- **Issues:** https://github.com/argus-xdr/argus/issues
- **Discussions:** https://github.com/argus-xdr/argus/discussions
- **Documentation:** [docs/](docs/)

---

**Happy testing on Arch Linux! 🚀**
