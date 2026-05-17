# Contributing to Argus XDR

Thank you for your interest in contributing. Argus XDR is designed to be a serious, production-grade platform and we hold contributions to the same standard. This document explains how to contribute effectively.

---

## Before You Start

### Read the architecture doc

Read `docs/architecture.md` before writing any code. Argus has specific opinions about layering (no cross-package imports that bypass the pipeline), storage (ClickHouse for signals, PostgreSQL for config), and auth (no shortcuts around the CSRF or JWT middleware). A contribution that bypasses these will not be merged.

### Check for an existing issue

Open a GitHub issue or check existing ones before spending time on a large change. For bug fixes, a minimal reproduction case in the issue helps significantly. For new features, a short design proposal (1-3 paragraphs) gets alignment before you write code.

### Small PRs merge faster

A 200-line PR with a clear problem statement gets reviewed and merged in a day. A 2,000-line PR with a vague description waits weeks. Split large changes into sequential PRs if possible.

---

## Development Setup

```bash
# Clone
git clone https://github.com/argusxdr/argus.git
cd argus

# Start dependencies
docker compose up -d clickhouse postgres redis

# Build
go build -o ./argus ./cmd/argus

# Run tests
go test ./...

# Verify health
./argus server &
curl http://localhost:8080/health
```

### Frontend

```bash
cd web
npm install
npm run dev     # http://localhost:3000
npm run build   # production build
npm run lint    # ESLint + TypeScript check
```

### Regenerating protobuf stubs

Requires `buf` CLI:

```bash
buf generate
# Outputs to gen/go/argus/v1/
```

---

## Contribution Workflow

1. **Fork** the repo on GitHub
2. **Branch** from `dev` (not `main`): `git checkout -b feat/my-change`
3. **Write code** — see conventions below
4. **Write tests** — PRs without tests for new behaviour are returned for revision
5. **Run the full test suite**: `go test ./... -race`
6. **Push** to your fork
7. **Open a PR** targeting `dev`

---

## Code Conventions

### Go

**Package layout is intentional.** The main packages are:
- `internal/ingest/` — receivers and queue only; no business logic
- `internal/pipeline/` — the 7-stage processing chain; each stage is a separate file
- `internal/storage/` — ClickHouse and PostgreSQL clients; no business logic
- `internal/auth/` — JWT, CSRF, RBAC; no ingest or storage imports
- `internal/trace/` — reconstruction and timeline; reads from storage, no writes
- `internal/baseline/` — async engine; reads from ClickHouse, writes to PostgreSQL and Redis
- `internal/api/` — HTTP handlers; thin layer over internal packages

Do not add business logic to `cmd/argus/`. Commands are thin wrappers that wire dependencies together.

**Error handling:**
- Return errors; do not log and swallow
- Use `fmt.Errorf("context: %w", err)` for wrapping
- Only log at the top-level handler (where the error stops propagating)

**Naming:**
- Exported names should be self-documenting: `BatchWriter`, not `BW`
- Acronyms follow Go convention: `HTTPAddr`, `gRPCServer`, `APIKey`
- Test files: `foo_test.go`, test functions: `TestFoo_WhenCondition_ExpectsResult`

**Logging:**
- Use the `zap.Logger` injected via struct field — never `log.Print`
- Log at `INFO` for lifecycle events, `DEBUG` for per-signal diagnostics, `WARN` for recoverable anomalies, `ERROR` for failures
- Never log secrets, API keys, JWTs, or user passwords

**Protobuf:**
- Changes to `proto/argus/v1/signal.proto` are breaking changes — field numbers must never be reused
- New fields get the next available field number
- Enum values use `SCREAMING_SNAKE_CASE` for the proto name; the wire name (used in protojson) is the value name without the type prefix (e.g., `DATA_CLASSIFICATION_INTERNAL` → wire: `"INTERNAL"`)
- Run `buf lint` and `buf breaking --against '.git#branch=main'` before submitting proto changes

### TypeScript / React

- Functional components only — no class components
- State: Zustand for global state, `useState` for local UI state, TanStack Query for server state
- Types: prefer explicit types over `any`; never use `// @ts-ignore` without a comment explaining why
- Charting: Apache ECharts only (not Recharts, Chart.js, or Plotly — these are explicitly rejected in spec)
- Styling: Tailwind classes only; no inline `style` props except for dynamic values that Tailwind can't express

### Testing

**Go — what to test:**
- Processing pipeline stages: unit test each stage with mock inputs
- Storage clients: integration tests against real ClickHouse/PostgreSQL/Redis (use the docker compose stack)
- Auth: unit test JWT generation/validation, CSRF token matching, TOTP verification
- API handlers: use `httptest.NewRecorder` for HTTP handler tests

**Minimum coverage for a PR:**
- New feature: at least unit tests for the happy path and one failure case
- Bug fix: a test that would have caught the bug
- Refactor: existing tests must continue to pass; add tests if coverage drops

**Do not write tests that:**
- Sleep for fixed durations (`time.Sleep` in tests is a test smell)
- Depend on test ordering
- Require network access that isn't gated by a build tag

---

## What Makes a Good PR

A good PR has four things:

**1. A clear problem statement**

"The `/api/v1/traces/recent` endpoint was being shadowed by the `{traceID}` wildcard route because `queryHandler.RegisterRoutes` ran first. Fixed by moving Phase 7 routes before the wildcard registration in `api.go`."

Not: "fixed route issue"

**2. The minimal change to solve the problem**

If you're fixing a bug, fix the bug. Don't refactor adjacent code in the same PR unless the refactor is necessary to make the fix safe.

**3. Tests**

See above. No tests → returned for revision.

**4. Updated documentation**

If your PR adds a new config option, add it to `docs/configuration.md`. If it adds a new API endpoint, document the request/response shape. If it changes a component's behaviour, update `docs/architecture.md`.

---

## PR Review Process

PRs targeting `dev` are reviewed by maintainers. Typical review turnaround is 2-3 days. The reviewer will:

- Check that the code solves the stated problem
- Verify tests cover the change
- Confirm no security regressions (auth bypass, injection, credential leak)
- Check that the PR doesn't introduce breaking changes without documentation

**Breaking changes** (proto field removal, API endpoint removal, auth scheme change, storage schema migration) require a maintainer sign-off and a migration note in the PR body.

---

## Security Vulnerabilities

Do not file public GitHub issues for security vulnerabilities. Email `security@argusxdr.io` with:
- A description of the vulnerability
- Steps to reproduce
- Your assessment of impact
- Whether you'd like credit in the advisory

We aim to respond within 48 hours and publish a fix within 7 days for critical issues.

---

## Versioning

Argus XDR follows semantic versioning (`MAJOR.MINOR.PATCH`):
- `PATCH` — bug fixes, no API or schema changes
- `MINOR` — new features, backward-compatible API additions
- `MAJOR` — breaking changes (proto field removal, auth scheme change, storage migration required)

The version is set in `cmd/argus/version.go`. Do not bump the version in a feature PR — maintainers manage version bumps as part of the release process.

---

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0, the same license as the rest of the project.
