# Development Workflow for Argus XDR

This document describes the development, testing, and deployment workflow for Argus XDR across multiple machines and operating systems.

## Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    GitHub (kairos-dev-kairos-ecl/argus)     │
├─────────────────────────────────────────────────────────────┤
│ main         → Production-ready code (tagged releases)      │
│ develop      → Staging for next release                     │
│ working/...  → Active development branches                  │
└─────────────────────────────────────────────────────────────┘
              ↑                                    ↑
              │                                    │
    ┌─────────┴─────────┐                 ┌──────┴──────────┐
    │                   │                 │                 │
┌─────────────┐   ┌──────────────┐  ┌─────────────┐  ┌──────────┐
│   Windows   │   │ Arch Linux   │  │   GitHub    │  │ Infisical│
│  (Dev)      │   │  (Test)      │  │  (CI/CD)    │  │(Secrets) │
│             │   │              │  │             │  │          │
│ • Code edit │   │ • Test       │  │ • Build     │  │ • Auth   │
│ • Build     │   │ • Validate   │  │ • Test      │  │ • DB PWs │
│ • Push      │   │ • Pull       │  │ • Deploy    │  │ • API    │
│             │   │              │  │             │  │          │
└─────────────┘   └──────────────┘  └─────────────┘  └──────────┘
```

## Branching Strategy

### Main Branches

| Branch | Purpose | Protection | Deploy |
|--------|---------|-----------|--------|
| `main` | Production releases | Requires PR review, all checks pass | Auto-deploy to prod |
| `develop` | Staging/next release | Requires PR review | Deploy to staging |
| `working/*` | Active development | None (developer's own branch) | Manual testing |

### Branch Naming

```
working/feature-name          # New feature work
working/backend-validation    # Backend testing harness
working/fix/issue-123         # Bug fix for GitHub issue #123
working/docs/setup-guide      # Documentation updates
```

## Development Workflow (Windows)

### 1. Create Feature Branch

```bash
git fetch origin
git checkout -b working/my-feature origin/working/backend-validation
```

### 2. Make Changes

```bash
# Edit code, commit atomically
git add <files>
git commit -m "feat: description of change"

# Or use GSD workflow for complex changes
/gsd:quick "add authentication to signals endpoint"
```

### 3. Test Locally

```bash
# Set up environment
infisical login
eval $(infisical export --env dev --format env)

# Run tests
go test ./...
make test

# Build Docker image
docker build -t argus:local .

# Or use validation harness
./test_harness/run_harness.sh
```

### 4. Push and Create PR

```bash
git push -u origin working/my-feature

# Create PR to develop branch (NOT main)
gh pr create --base develop --title "Feature: ..." --body "..."
```

## Testing Workflow (Arch Linux)

### 1. Pull Latest Working Branch

```bash
git fetch origin
git checkout working/backend-validation
git pull origin working/backend-validation
```

### 2. Set Up Test Environment

```bash
# Install dependencies
yay -S docker docker-compose python infisical

# Start Docker daemon
sudo systemctl start docker

# Set up Infisical
infisical login
```

### 3. Run Validation Harness

```bash
# Fetch test credentials from Infisical
infisical export --env test > /tmp/test.env
source /tmp/test.env

# Run full validation
./test_harness/run_harness.sh

# Or run with Docker Compose directly
docker-compose -f docker-compose-test.yml up -d
python test_harness/instrumented_llm.py
python test_harness/validate_capture.py
docker-compose -f docker-compose-test.yml down -v
```

### 4. Report Results

If validation passes:
```bash
# Comment on PR
gh pr comment <PR_NUMBER> --body "✓ Backend validation passed on Arch Linux"
```

If issues found:
```bash
# Create issue for developer
gh issue create \
  --title "Backend validation: signal gap on layer 5" \
  --body "Arch test run found: $(cat /tmp/validation_results.md)"
```

## CI/CD Pipeline (GitHub Actions)

### Triggers

- **Push to `working/backend-validation`** → Run backend validation
- **Push to `develop`** → Build + test + deploy to staging
- **Release tag on `main`** → Build + test + deploy to production

### Workflow: `backend-validation.yml`

```
Trigger: Push to working/* or PR to develop/main
    ↓
1. Checkout code
2. Build Argus binary (Go)
3. Install Python dependencies
4. Start Argus API server (with ARGUS_TEST_MODE=true)
5. Mock llama.cpp server (or skip on resource-constrained runner)
6. Run signal validation harness
7. Validate signal capture from ClickHouse
8. Upload validation report as artifact
9. Comment results on PR
```

**Secrets Used:**
- `INFISICAL_TOKEN` → Fetch test credentials

**Artifacts:**
- `validation-report/` → Harness output, validation results

## Secrets Management (Infisical)

### Environments

| Environment | Use Case | Accessible By |
|-------------|----------|---------------|
| `dev` | Local development (Windows) | Developer only |
| `test` | Arch test machine | Developer only |
| `ci` | GitHub Actions workflows | GitHub service token |

### Secret Rotation

```bash
# Every month:
1. Log into Infisical UI
2. Rotate database passwords (mark old as revoked)
3. Regenerate service tokens
4. Update GitHub Secrets
5. Notify team

# Example:
infisical secrets rotate --secret-name ARGUS_DATABASE_POSTGRES_DSN
```

## Common Workflows

### "I've finished implementing a feature"

```bash
git push origin working/my-feature
gh pr create --base develop

# Wait for CI to pass
# On Arch: pull and test
git checkout working/my-feature
./test_harness/run_harness.sh

# Approve and merge PR
gh pr merge --squash
```

### "I need to test across Windows and Arch"

```bash
# Windows: commit and push
git push origin working/my-feature

# Arch: pull latest
git pull origin working/my-feature

# Run validation harness
./test_harness/run_harness.sh

# If issues: push fix from either machine (it syncs via git)
```

### "I found a bug on Arch, need to fix it on Windows"

```bash
# Arch: document issue
echo "Bug: signals not captured on L7" >> /tmp/bug_report.md

# Windows: fetch latest
git pull origin working/backend-validation

# Create fix branch from working branch
git checkout -b working/fix/layer-7

# Implement fix, test
make test
./test_harness/run_harness.sh

# Push
git push origin working/fix/layer-7

# Arch: pull and verify
git pull origin working/fix/layer-7
./test_harness/run_harness.sh
```

### "I need to merge to develop"

```bash
# Ensure working branch is fully tested and documented
git log origin/develop..working/my-feature --oneline

# Create PR
gh pr create --base develop --title "Feature: ..." 

# GitHub Actions runs: build + test
# CI passes ✓

# Code review ✓

# Merge
gh pr merge --squash
```

## Troubleshooting

### "Docker Compose fails on Arch"

```bash
# Check Docker daemon
sudo systemctl status docker
sudo systemctl start docker

# Check compose version
docker-compose --version

# Try with docker compose (newer) instead of docker-compose
docker compose -f docker-compose-test.yml up
```

### "Infisical token expired"

```bash
# Re-authenticate
infisical logout
infisical login

# Regenerate service token in UI if needed
# Update INFISICAL_TOKEN in GitHub Secrets
```

### "Signal validation fails, unsure why"

```bash
# Run with debug output
ARGUS_LOGGING_DEV=true ./test_harness/run_harness.sh

# Check logs
docker compose -f docker-compose-test.yml logs argus-server-test | tail -50

# Inspect ClickHouse directly
docker exec argus-clickhouse-test clickhouse-client --query "SELECT COUNT(*) FROM signals"
```

## Performance Notes

- **Windows → Arch Push**: ~1-5 sec (git)
- **Arch → Windows Pull**: ~1-5 sec (git)
- **Backend Validation (full)**: ~2-3 min (including Docker startup)
- **CI/CD (GitHub)**: ~5-10 min (build + test)
- **Signal ingestion (harness)**: ~30 sec (5 prompts × ~6 sec each)

## Next Steps

1. ✓ Infisical account created with argus-xdr project
2. ✓ Secrets defined for dev/test/ci environments
3. ✓ GitHub Actions workflow configured
4. ✓ Development branch pushed (working/backend-validation)
5. → Start feature work from `working/backend-validation` branch
6. → Test on Arch before merging to `develop`
7. → Merge to `develop` for staging validation
8. → Tag and merge to `main` for release

## References

- [Git Workflow](https://git-scm.com/book/en/v2)
- [GitHub Actions](https://docs.github.com/en/actions)
- [Infisical Secrets](./INFISICAL_SETUP.md)
- [Backend Validation Harness](./test_harness/README.md)
