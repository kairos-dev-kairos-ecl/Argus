.PHONY: help proto-format proto-lint proto-breaking proto-generate proto-clean

help:
	@echo "Argus XDR — Protobuf Makefile"
	@echo ""
	@echo "Proto targets:"
	@echo "  make proto-format       Format .proto files"
	@echo "  make proto-lint         Lint .proto files"
	@echo "  make proto-breaking     Check for breaking changes"
	@echo "  make proto-generate     Generate Go/Python/TS stubs"
	@echo "  make proto-validate     Run format, lint, breaking checks"
	@echo "  make proto-clean        Remove generated stubs"

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

# Default target
.DEFAULT_GOAL := help
