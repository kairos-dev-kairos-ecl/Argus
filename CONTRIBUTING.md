# Contributing to Argus XDR

Thank you for your interest in contributing to Argus XDR! This document provides guidelines and instructions for contributing.

## Code of Conduct

Be respectful, inclusive, and constructive. We're building an open-source security platform for everyone.

## How to Contribute

### Reporting Bugs

1. **Check existing issues** — Your bug may already be reported
2. **Create a detailed report** including:
   - OS, Go/Node/Python version
   - Steps to reproduce
   - Expected vs actual behavior
   - Logs or error messages
   - Minimal reproducible example

### Suggesting Features

1. **Search discussions** for similar ideas
2. **Create a discussion** or issue with:
   - Clear use case
   - Benefits and rationale
   - Potential implementation approach
3. **Wait for feedback** before implementing

### Code Contributions

#### Setup

```bash
git clone https://github.com/argus-xdr/argus.git
cd argus

# Backend
go mod tidy
go build ./cmd/argus

# Frontend
cd web
npm install
npm run dev
```

#### Branch Workflow

1. **Fork the repository** (if needed)
2. **Create a feature branch:**
   ```bash
   git checkout -b feature/your-feature-name
   ```
3. **Make changes** following code style guidelines
4. **Write tests** for new functionality
5. **Commit with clear messages:**
   ```bash
   git commit -m "feat(component): description of change"
   ```
6. **Push and open a pull request**

#### Commit Message Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`  
Scope: `auth`, `api`, `ui`, `sdk`, etc.  
Subject: Imperative mood, no period

#### Code Style

**Go:**
- `gofmt` formatting
- Meaningful variable names
- Comments for exported functions
- Avoid globals

**TypeScript/React:**
- `eslint` rules configured in project
- `prettier` for formatting
- JSDoc comments for components
- Functional components preferred

**Python:**
- `black` and `isort` formatting
- Type hints
- Docstrings

#### Testing Requirements

- **Unit tests:** New code should have unit tests
- **Integration tests:** API changes should have integration tests
- **E2E tests:** UI changes should have Playwright tests

```bash
# Run tests
go test ./...
cd web && npm test
python -m pytest test_harness/

# With coverage
go test -cover ./...
```

#### Documentation

- Update README.md if user-facing
- Add docstrings/comments for implementation
- Update relevant .md files in docs/
- Include examples for new features

### Pull Request Process

1. **Self-review** before requesting review
2. **Fill out PR template** completely
3. **Link related issues** (#123)
4. **Run tests locally** and ensure they pass
5. **Squash commits** if requested by reviewer
6. **Wait for approval** from maintainers

#### PR Title Format

```
[type](scope): description

Examples:
[feat](api): add Kairos policy evaluation
[fix](ui): correct contrast ratio in dark theme
[docs](sdk): add Python decorator examples
```

### Development Practices

#### Architecture Guidelines

- **Fail open:** Services degrade gracefully when dependencies fail
- **Observability:** Add metrics and logging for production visibility
- **Security:** Validate inputs, use RBAC, encrypt sensitive data
- **Performance:** Keep SDK overhead <5ms, API latency <100ms

#### Testing Strategy

- **Unit tests:** Logic, edge cases, error handling
- **Integration tests:** API contracts, database interactions
- **Smoke tests:** End-to-end happy paths
- **Load tests:** Performance under stress

#### Performance Considerations

- Benchmark changes: `go test -bench`
- Profile memory: `go tool pprof`
- Monitor dashboard: `curl http://localhost:8080/metrics`
- Database queries: Use EXPLAIN, check indexes

### Release Process

1. **Version bump** (semantic versioning)
2. **Update CHANGELOG.md**
3. **Create GitHub release** with notes
4. **Build artifacts** (binaries, Docker images)
5. **Publish** to registries

## Project Structure

```
argus/
├── cmd/argus/          # CLI entry points
├── internal/           # Core implementation
├── proto/              # Protocol buffers
├── sdk/                # Language SDKs
├── web/                # React frontend
├── deployments/        # Helm, systemd, etc.
├── docs/               # Documentation
├── test_harness/       # Attack scenario testing
└── tests/              # Integration tests
```

## Code Review Checklist

**For Contributors:**
- [ ] Code follows style guidelines
- [ ] Tests added/updated
- [ ] Documentation updated
- [ ] No breaking changes (unless major release)
- [ ] Performance impact assessed

**For Reviewers:**
- [ ] Code review for correctness and style
- [ ] Tests are adequate
- [ ] Docs are clear
- [ ] No security issues
- [ ] Approved or feedback provided

## Getting Help

- **Questions:** Use GitHub Discussions
- **Issues:** Open an issue for bugs
- **Security:** Report privately to [security contact]
- **Chat:** Join our Discord (link in README)

## Resources

- [Developer Guide](docs/) — Architecture and internals
- [API Documentation](docs/API_REFERENCE.md) — Endpoints
- [SDK Guides](docs/SDK_GUIDE.md) — Language-specific integration

---

**Thank you for contributing to Argus XDR! 🙏**
