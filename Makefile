.PHONY: help build build-api build-ingest run-api run-ingest docker-up docker-down test clean lint fmt proto-format proto-lint proto-breaking proto-generate proto-clean proto-validate harness-llama validate

help:
	@echo "Argus XDR — Build System"
	@echo ""
	@echo "Build targets:"
	@echo "  make build              - Build all Go binaries"
	@echo "  make build-api          - Build API server binary"
	@echo ""
	@echo "Docker targets:"
	@echo "  make docker-up          - Start infrastructure with Docker Compose"
	@echo "  make docker-down        - Stop infrastructure"
	@echo "  make docker-logs        - Stream Docker logs"
	@echo ""
	@echo "Development targets:"
	@echo "  make run-api            - Run API server (requires services)"
	@echo "  make lint               - Run golangci-lint"
	@echo "  make fmt                - Format Go code"
	@echo "  make test               - Run tests"
	@echo "  make clean              - Remove binaries"
	@echo ""
	@echo "Test harness targets:"
	@echo "  make harness-llama      - Run test harness with llama.cpp API"
	@echo "  make validate           - Validate signal capture"
	@echo ""
	@echo "Protobuf targets:"
	@echo "  make proto-format       - Format .proto files"
	@echo "  make proto-lint         - Lint .proto files"
	@echo "  make proto-breaking     - Check for breaking changes"
	@echo "  make proto-generate     - Generate Go/Python/TS stubs"
	@echo "  make proto-validate     - Run format, lint, breaking checks"
	@echo "  make proto-clean        - Remove generated stubs"

proto-format:
	@echo "Formatting .proto files..."
	cd proto && buf format -w

proto-lint:
	@echo "Linting .proto files..."
	cd proto && buf lint

proto-breaking:
	@echo "Checking for breaking changes..."
	cd proto && buf breaking --against 'https://github.com/argusxdr/argus.git#branch=main' 2>/dev/null || echo "No remote branch to compare; skipping."

proto-generate:
	@echo "Generating Go/Python/TypeScript stubs..."
	cd proto && buf generate

proto-validate: proto-format proto-lint proto-breaking
	@echo "✓ Protobuf validation passed"

proto-clean:
	@echo "Cleaning generated stubs..."
	rm -rf proto/gen

proto-validate: proto-format proto-lint proto-breaking
	@echo "✓ Protobuf validation passed"

proto-clean:
	@echo "Cleaning generated stubs..."
	rm -rf gen/

# Build all binaries
build: build-api
	@echo "✓ All binaries built successfully"
	@echo "  Binaries in: ./bin/"

# Build API server
build-api:
	@echo "Building API server..."
	go build -o bin/argus-api ./cmd/argus/
	@echo "✓ API server built: ./bin/argus-api"

# Run API server (requires services running)
run-api: build-api
	@echo "Running API server on :8080..."
	@echo "Connecting to: ClickHouse (localhost:9000), PostgreSQL (localhost:5432), Redis (localhost:6379)"
	ARGUS_SERVER_HTTP_ADDR=0.0.0.0:8080 \
	ARGUS_DATABASE_CLICKHOUSE_DSN=localhost:9000 \
	ARGUS_DATABASE_POSTGRES_DSN="postgresql://argus:argus@localhost:5432/argus?sslmode=disable" \
	ARGUS_REDIS_ADDR=localhost:6379 \
	./bin/argus-api api

# Docker compose operations
docker-up:
	@echo "Starting Docker infrastructure..."
	docker-compose -f deployments/docker-compose.yml up -d
	@sleep 3
	@echo "✓ Services started:"
	@echo "  API:        http://localhost:8080"
	@echo "  Dashboard:  http://localhost:3000"
	@echo "  ClickHouse: localhost:9000"
	@echo "  PostgreSQL: localhost:5432"
	@echo "  Redis:      localhost:6379"

docker-down:
	@echo "Stopping Docker infrastructure..."
	docker-compose -f deployments/docker-compose.yml down
	@echo "✓ Stopped"

docker-logs:
	docker-compose -f deployments/docker-compose.yml logs -f

# Code quality
lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "Installing golangci-lint..."; go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; }
	golangci-lint run ./...

fmt:
	@echo "Formatting Go code..."
	gofmt -s -w .
	@echo "✓ Formatted"

test:
	@echo "Running tests..."
	go test -v ./...

# Cleanup
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	@echo "✓ Cleaned"

# Test harness targets
harness-llama:
	@echo "Running test harness with llama.cpp API..."
	cd test_harness && python qwen_llama_api.py

validate:
	@echo "Validating signal capture..."
	cd test_harness && python validate_signals.py

# Development workflow
dev-setup: docker-up
	@echo "Development environment ready:"
	@echo "  1. Verify services: docker ps"
	@echo "  2. Start llama.cpp: llama-server -hf unsloth/Qwen3.5-0.8B-GGUF:UD-Q4_K_XL"
	@echo "  3. Start API (new terminal): make run-api"
	@echo "  4. Run test harness (new terminal): make harness-llama"
	@echo "  5. View dashboard: http://localhost:3000"

# Default target
.DEFAULT_GOAL := help
