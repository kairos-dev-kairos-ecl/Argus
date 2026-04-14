# Multi-stage Docker build for Argus XDR
# Proto files are pre-generated locally (gen/ directory committed)
# This keeps Docker simple and fast

# Stage 1: Build the binary
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code including pre-generated proto files
COPY . .

# Verify proto files exist
RUN ls -la gen/go/argus/v1/ && echo "✓ Proto files ready"

# Build the binary (static, no CGO)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o /argus ./cmd/argus

# Stage 2: Minimal runtime image
FROM alpine:3.19

# Install runtime dependencies (curl for health checks, ca-certificates for TLS)
RUN apk --no-cache add ca-certificates curl

# Copy binary from builder
COPY --from=builder /argus /usr/local/bin/argus

# Copy built-in detection rules
COPY --from=builder /app/internal/rules /app/internal/rules

# Set working directory so relative paths work
WORKDIR /app

# Health check
HEALTHCHECK --interval=10s --timeout=5s --retries=5 \
  CMD curl -f http://localhost:8080/health || exit 1

# Default entry point and command
ENTRYPOINT ["argus"]
CMD ["server", "start"]
