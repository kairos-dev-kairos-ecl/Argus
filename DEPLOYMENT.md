# Argus XDR - Deployment Guide

**Project Size:** 4.0 MB (source only, no node_modules, .git, or build artifacts)

## 📦 What's Included

This is a **clean, production-ready source distribution**. All unnecessary files have been removed:

- ✅ Go source code (cmd/, internal/, proto/)
- ✅ React + TypeScript frontend (web/)
- ✅ Python & TypeScript SDKs (sdk/)
- ✅ Docker Compose configuration
- ✅ Kubernetes/systemd deployment templates
- ✅ Complete documentation
- ✅ Test harness & reference applications
- ✅ Brand assets (logo, color palette)

## 🚀 Quick Start (Arch Linux)

### 1. Copy to Your Machine
```bash
# Option A: Via USB/network share
cp -r /path/to/argus ~/argus-xdr
cd ~/argus-xdr

# Option B: Via tar archive
tar -xzf argus-xdr-clean.tar.gz
cd argus-xdr
```

### 2. Install Dependencies
```bash
# Docker + Docker Compose (required)
sudo pacman -S --noconfirm docker docker-compose
sudo systemctl enable docker
sudo systemctl start docker
sudo usermod -aG docker $USER
# Log out and back in

# Go (for development/building)
sudo pacman -S --noconfirm go

# Node.js (for frontend)
sudo pacman -S --noconfirm nodejs npm
```

### 3. Start Services
```bash
docker-compose up -d
```

### 4. Verify Deployment
```bash
# Check all services are running
docker-compose ps

# View logs
docker-compose logs -f argus-backend
docker-compose logs -f argus-frontend
```

### 5. Access Dashboard
```
http://localhost:9090/setup
```

Create your first admin account and start using Argus.

## 📋 Directory Structure

```
argus-xdr/
├── README.md                    # Project overview
├── ARCH_LINUX_SETUP.md         # Detailed Arch guide
├── RELEASE_NOTES.md            # v1.0.0 completion summary
├── CLAUDE.md                   # Development guidelines
├── CONTRIBUTING.md             # Contribution guidelines
│
├── docker-compose.yml          # Local dev deployment
├── Dockerfile                  # Backend image
├── Makefile                    # Build targets
│
├── cmd/argus/                  # CLI entry points
├── internal/                   # Backend implementation
│   ├── auth/                   # JWT/RBAC
│   ├── signal/                 # Ingestion pipeline
│   ├── detection/              # Rules engine
│   ├── baseline/               # Anomaly detection
│   └── ...
│
├── web/                        # React frontend
│   ├── src/
│   │   ├── components/         # UI components
│   │   ├── pages/              # Page routes
│   │   ├── stores/             # Zustand state
│   │   └── lib/                # Design tokens
│   └── package.json
│
├── sdk/                        # Language SDKs
│   ├── python/                 # Python @observe decorator
│   ├── typescript/             # TypeScript middleware
│   └── otel/                   # OpenTelemetry bridge
│
├── proto/                      # Protocol Buffers
│   ├── argus/
│   │   ├── signal.proto        # Core signal schema
│   │   ├── api.proto           # API definitions
│   │   └── rules.proto         # Detection rules
│
├── deployments/                # Production templates
│   ├── helm/                   # Kubernetes
│   ├── systemd/                # Linux services
│   └── launchd/                # macOS services
│
├── tests/                      # Integration tests
├── test_harness/               # Attack scenario testing
├── benchmarks/                 # Performance tests
├── docs/                       # Documentation
│   ├── USER_GUIDE.md
│   ├── SDK_GUIDE.md
│   ├── TEST_HARNESS.md
│   └── PRODUCTION_CHECKLIST.md
│
├── go.mod / go.sum             # Go dependencies
└── assets/                     # Brand assets (logo, etc.)
```

## 🧪 Testing Locally

### Start with Docker Compose (Recommended)
```bash
# Backend (Go), Frontend (React), ClickHouse, PostgreSQL, Redis
docker-compose up -d

# Wait ~10 seconds for services to initialize
sleep 10

# View logs
docker-compose logs -f
```

### Run Test Harness
```bash
cd test_harness
pip install -r requirements.txt

# Run baseline scenario
python runner.py --scenario-set A

# Run prompt injection tests
python runner.py --scenario-set B

# Run all scenarios
python runner.py --all-scenarios
```

### Integrate with Your App
```python
from argus import observe, Layer, Category

@observe(Layer.L7, Category.RETRIEVAL_SEARCH)
def my_search_function(query):
    # Your code here
    return results
```

## 🛠️ Building from Source

### Backend
```bash
go mod download
go build -o argus ./cmd/argus
./argus --version
```

### Frontend
```bash
cd web
npm install
npm run build
# Outputs to web/dist/
```

### SDKs
```bash
# Python
cd sdk/python
pip install -e .

# TypeScript
cd sdk/typescript
npm install
npm run build
```

## 🔧 Configuration

All services are configured via `docker-compose.yml`:

- **Argus Backend:** Port 8080 (API), gRPC on 50051
- **Frontend:** Port 9090 (served by backend)
- **ClickHouse:** Port 9000 (native)
- **PostgreSQL:** Port 5432
- **Redis:** Port 6379

Override with environment variables:
```bash
ARGUS_API_PORT=8888 docker-compose up -d
```

## 📊 Monitoring

### Dashboard Metrics
```
http://localhost:8080/metrics
```

### View Signals in Real-Time
```bash
# Get dashboard auth token
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"..."}'

# Query signals
curl http://localhost:8080/api/v1/signals?limit=10 \
  -H "Authorization: Bearer $TOKEN"
```

## ⚠️ Production Checklist

Before deploying to production:

- [ ] Review [PRODUCTION_CHECKLIST.md](docs/PRODUCTION_CHECKLIST.md)
- [ ] Configure TLS certificates
- [ ] Set strong admin password
- [ ] Enable RBAC with service accounts
- [ ] Configure log aggregation
- [ ] Set up monitoring/alerting
- [ ] Run load tests (see benchmarks/)
- [ ] Test failover scenarios

## 🔐 Security

- JWT tokens (HttpOnly cookies)
- Role-based access control (3 roles: admin, analyst, viewer)
- Audit logging (immutable)
- Password security (Bcrypt cost 12)
- CSRF protection
- XSS defense

See [CONTRIBUTING.md](CONTRIBUTING.md) for security guidelines.

## 📚 Documentation

| Document | Purpose |
|----------|---------|
| [README.md](README.md) | Overview & quick start |
| [ARCH_LINUX_SETUP.md](ARCH_LINUX_SETUP.md) | Arch-specific guide |
| [docs/USER_GUIDE.md](docs/USER_GUIDE.md) | Operations & workflows |
| [docs/SDK_GUIDE.md](docs/SDK_GUIDE.md) | SDK integration |
| [docs/TEST_HARNESS.md](docs/TEST_HARNESS.md) | Attack scenario testing |
| [docs/PRODUCTION_CHECKLIST.md](docs/PRODUCTION_CHECKLIST.md) | Production deployment |

## 🆘 Troubleshooting

**Docker services won't start:**
```bash
# Check logs
docker-compose logs

# Verify ports are free
netstat -tulpn | grep -E ':(8080|9090|5432|6379|9000)'

# Reset (WARNING: deletes data)
docker-compose down -v
docker-compose up -d
```

**Frontend shows "Disconnected":**
- Backend may not be running (`docker-compose logs argus-backend`)
- Check WebSocket connection to `ws://localhost:8080/ws`

**Test harness fails:**
- Verify ClickHouse is running: `docker-compose ps clickhouse`
- Check backend logs: `docker-compose logs argus-backend`
- Ensure test data was ingested

## ✅ Verification Checklist

After deployment, verify:

- [ ] Dashboard loads at http://localhost:9090
- [ ] Can create admin account
- [ ] Can register test application
- [ ] Can ingest test signals
- [ ] Can view signals in dashboard
- [ ] Detection rules load correctly
- [ ] Test harness completes successfully

---

**Ready to deploy. Happy testing! 🚀**
