# Infisical Secrets Management Setup

This guide covers setting up Infisical for managing authentication tokens, certificates, and API credentials for Argus XDR development and testing.

## Why Infisical?

- **Single source of truth** for secrets across dev/test/prod environments
- **No secrets in code** — especially important for open-source projects
- **Rotation-friendly** — update secrets without redeploying
- **Audit trail** — track who accessed what secrets and when
- **GitHub integration** — native support for Actions workflows

## Architecture

```
Developer Machine (Windows/Arch)
    ↓
    └─→ Infisical CLI
        ├─ API Key (personal)
        ├─ LLM credentials (test)
        └─ Database passwords (test)
    ↓
GitHub Actions (working/backend-validation branch)
    ├─ Fetch secrets via Infisical API
    ├─ Run backend validation harness
    └─ Revoke API key after job completes (optional)
```

## Setup Steps

### 1. Create Infisical Account

```bash
# Sign up at https://infisical.com
# Free tier includes:
# - Unlimited projects
# - 10 team members
# - Full API access
# - Audit logs
```

### 2. Create Infisical Organization & Project

```bash
# Via UI: https://app.infisical.com
# Create project: "argus-xdr"
# Create environments:
#   - dev (your local machine)
#   - ci (GitHub Actions)
#   - test (Arch test machine)
```

### 3. Define Secrets

In the Infisical UI, create these secrets:

**Development Secrets** (dev environment):
```
ARGUS_TEST_MODE = true
ARGUS_DATABASE_POSTGRES_DSN = postgresql://argus:argus@localhost:5433/argus?sslmode=disable
ARGUS_DATABASE_CLICKHOUSE_DSN = localhost:9001
ARGUS_REDIS_ADDR = localhost:6380
LLAMA_API_KEY = (if using remote llama.cpp)
```

**CI/CD Secrets** (ci environment):
```
ARGUS_TEST_MODE = true
ARGUS_DATABASE_POSTGRES_DSN = postgresql://argus:argus@postgres:5432/argus?sslmode=disable
ARGUS_DATABASE_CLICKHOUSE_DSN = clickhouse:9000
ARGUS_REDIS_ADDR = redis:6379
DOCKER_REGISTRY_TOKEN = (if pushing images)
GITHUB_TOKEN = (auto-injected by GitHub)
```

**Test Machine Secrets** (test environment):
```
ARGUS_TEST_MODE = true
ARGUS_DATABASE_POSTGRES_DSN = postgresql://argus:argus@arch-machine:5432/argus?sslmode=disable
ARGUS_DATABASE_CLICKHOUSE_DSN = arch-machine:9000
ARGUS_REDIS_ADDR = arch-machine:6379
```

### 4. Install Infisical CLI

**On Windows:**
```bash
# Using Scoop
scoop install infisical

# Or using Chocolatey
choco install infisical

# Or download: https://infisical.com/docs/cli/overview
```

**On Arch Linux:**
```bash
yay -S infisical-bin
# or
pacman -S infisical  # if in AUR
```

### 5. Authenticate with Infisical

```bash
# Login to Infisical
infisical login

# You'll be prompted to:
# 1. Visit https://infisical.com/login
# 2. Copy the device code
# 3. Approve access from CLI

# Verify login
infisical whoami
```

### 6. Export Secrets to Environment

```bash
# Pull secrets for dev environment
infisical export --env dev > .env.dev

# Source them
source .env.dev  # macOS/Linux
# or
Set-Content -Path .env.dev.ps1 -Value (infisical export --env dev --format powershell)
. .env.dev.ps1  # PowerShell
```

### 7. Run Tests with Infisical

```bash
# Local testing (any environment):
infisical run --env dev -- ./test_harness/run_harness.sh

# Or explicitly pull and use:
eval $(infisical export --env dev --format env)
./test_harness/run_harness.sh

# On Arch test machine:
eval $(infisical export --env test --format env)
docker-compose -f docker-compose-test.yml up -d
./test_harness/run_harness.sh
```

### 8. GitHub Actions Integration

**Step 1: Create Infisical API Key**

```bash
# In Infisical UI:
# Settings → API Keys → Create Service Token
# Name: "github-actions-argus"
# Permissions: Read secrets
# Store the token securely
```

**Step 2: Add GitHub Secret**

```bash
# In GitHub UI:
# Settings → Secrets and variables → Actions
# New repository secret:
# Name: INFISICAL_TOKEN
# Value: (paste service token)
```

**Step 3: Update Workflow to Use Infisical**

In `.github/workflows/backend-validation.yml`:

```yaml
- name: Fetch secrets from Infisical
  env:
    INFISICAL_TOKEN: ${{ secrets.INFISICAL_TOKEN }}
  run: |
    # Install Infisical CLI
    curl -1sLf 'https://dl.infisical.com/install.sh' | sudo bash
    
    # Authenticate
    echo "$INFISICAL_TOKEN" | infisical login --service-token -
    
    # Export secrets for CI environment
    eval $(infisical export --env ci --format env)

- name: Run backend validation
  run: |
    # Secrets now available as env vars
    ./test_harness/run_harness.sh
```

### 9. Rotate Secrets

```bash
# Update secret in Infisical UI, then:

# All environments automatically get new value on next fetch
infisical export --env dev | grep SECRET_NAME

# For CI: workflow automatically fetches latest on next run
# For local: just re-run infisical login and export

# Revoke old tokens in Infisical UI
```

## Best Practices

1. **Never commit `.env` files**
   ```bash
   # Add to .gitignore
   echo ".env*" >> .gitignore
   echo "!.env.example" >> .gitignore
   ```

2. **Use Infisical CLI for local development**
   ```bash
   # Instead of: export ARGUS_TEST_MODE=true
   # Use: infisical run --env dev -- command
   ```

3. **Minimize token scope**
   - Create separate service tokens for different apps
   - Use "Read-only" permissions where possible
   - Rotate keys regularly (monthly)

4. **Audit secret access**
   - Review Infisical audit logs monthly
   - Check GitHub Actions logs for secret usage
   - Alert on unauthorized access attempts

5. **Environment isolation**
   - Keep dev/test/prod secrets in separate environments
   - Use different database passwords per environment
   - Different API keys for each service (llama.cpp, Kairos, etc.)

## Troubleshooting

**"Invalid service token"**
```bash
# Regenerate service token in Infisical UI
# Update INFISICAL_TOKEN in GitHub Secrets
```

**"Secret not found"**
```bash
# Verify secret exists in Infisical UI
# Check correct environment is selected
# infisical export --env dev --list  # See all secrets
```

**"Timeout connecting to Infisical"**
```bash
# Check network connectivity
# Verify service token hasn't expired (30-day default)
# Check Infisical status: https://status.infisical.com
```

## Next Steps

1. Set up Infisical account at https://infisical.com
2. Create "argus-xdr" project with dev/ci/test environments
3. Define secrets (database, API keys, etc.)
4. Install CLI and authenticate
5. Update GitHub Actions workflow
6. Test locally: `infisical run --env dev -- ./test_harness/run_harness.sh`
7. Verify CI workflow fetches and uses secrets correctly

## References

- [Infisical Docs](https://infisical.com/docs)
- [Infisical CLI Guide](https://infisical.com/docs/cli/overview)
- [GitHub Actions Integration](https://infisical.com/docs/integrations/cicd/github-actions)
- [Service Tokens](https://infisical.com/docs/api-reference/service-tokens)
