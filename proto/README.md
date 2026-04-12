# Argus XDR — Protobuf Schema

This directory contains the Argus XDR protobuf schema definitions — the contract that all SDKs, servers, and external consumers depend on.

## Structure

```
proto/
├── buf.yaml              # Buf configuration: module definition, linting rules, breaking policy
├── buf.gen.yaml          # Buf codegen config: Go, Python, TypeScript generation targets
├── argus/v1/
│   ├── signal.proto      # ArgusSignal envelope + per-layer context messages (L1–L10)
│   ├── service.proto     # gRPC service definitions (Ingest, Query, Config, Health)
│   └── categories.proto  # Signal category enum (canonical L1–L10 subcategories)
├── gen/                  # Generated stubs (checked into git for reproducibility)
│   ├── go/               # Go server stubs (protoc-gen-go + protoc-gen-go-grpc)
│   ├── python/           # Python client stubs (protoc-gen-python)
│   └── typescript/       # TypeScript client stubs (ts-proto)
└── README.md             # This file
```

## Quick Start

### Prerequisites

```bash
# Install buf CLI
curl -sSL https://github.com/bufbuild/buf/releases/download/v1.30.0/buf-Linux-x86_64.tar.gz | tar -xz

# Install Go code generators
go install github.com/grpc/grpc-go/cmd/protoc-gen-go-grpc@latest

# Install Python code generator
pip install grpcio-tools

# Install TypeScript code generator
npm install -g @bufbuild/protoc-gen-ts
```

### Validate Schema

```bash
cd proto

# Format .proto files
buf format -w

# Lint schema
buf lint

# Check for breaking changes (against main branch)
buf breaking --against 'https://github.com/argusxdr/argus.git#branch=main'

# Generate stubs for all consumers
buf generate
```

Or use the Makefile from the repo root:

```bash
make proto-validate
make proto-generate
```

## Schema Design Principles

### No `map<string, any>`

Per-layer context fields are **strongly-typed protobuf messages**, not generic maps. This enables:
- IDE autocomplete on context fields
- Type safety at compile time
- Efficient storage in ClickHouse
- Clear documentation of what each layer can signal

Example (✓ Good):
```protobuf
message ContextL5 {
  int32 output_tokens = 1;
  float mean_logprob = 2;
  repeated LogProbEntry logprobs = 3;
}
```

Example (✗ Bad):
```protobuf
message ContextL5 {
  map<string, google.protobuf.Any> context = 1;
}
```

### Backward Compatibility First

Schema changes are governed by `buf breaking` CI enforcement:
- ✓ Add optional field → allowed
- ✓ Add oneof variant → allowed
- ✓ Mark field deprecated → allowed
- ✗ Remove field → blocked
- ✗ Rename field → blocked
- ✗ Change field type → blocked

**Never delete a field.** Reserve it instead:

```protobuf
message ArgusSignal {
  // string old_field = 5;  // Removed in v1.5.0
  reserved 5;
  reserved "old_field";
}
```

See `SCHEMA_EVOLUTION.md` for detailed versioning strategy.

## File Organization

### signal.proto

The core envelope that every signal must conform to:
- **Identity**: `signal_id` (ULID), `trace_id`, `span_id`, `parent_span_id`
- **Source**: app metadata, SDK version, environment
- **Classification**: layer, category, severity
- **Temporal**: timestamp (nanosecond precision), duration, ingestion time
- **Context**: `oneof` variants for L1–L10 per-layer payloads
- **Relationships**: correlated signal IDs, incident/session/conversation/user IDs
- **Provider**: model name, version, region
- **Enrichment**: threat intel, GeoIP, baseline z-score, risk score
- **Governance**: data classification, retention policy, PII detection flag

### service.proto

gRPC service definitions:
- **IngestService**: `StreamSignals(stream ArgusSignal) → IngestResponse`
  - Client-streaming: SDK batches signals in one RPC
  - Returns acknowledgement with accept/reject counts and rejection reasons

- **QueryService**: Query stored signals
  - `GetSignals(QueryRequest) → stream ArgusSignal` — filter by time, layer, app, severity
  - `GetTrace(TraceRequest) → TraceResponse` — reconstruct full trace
  - `GetAlerts(AlertQueryRequest) → stream Alert` — query alerts

- **ConfigService**: Manage platform configuration
  - Rule CRUD, settings updates, environment configuration

- **HealthService**: Platform health status
  - `Check()` and `Watch()` for Kubernetes liveness/readiness probes

### categories.proto

Enum of canonical signal categories. Maps to the `category` string field in ArgusSignal.

Examples:
- `RETRIEVAL_SEARCH = 702` → category string `"retrieval.search"`
- `AGENT_TOOL_CALL = 801` → category string `"agent.tool_call"`
- `SAFETY_OUTPUT_FILTER = 602` → category string `"safety.output_filter"`

Used by detection rules, dashboard filters, and queries to refer to specific signal types unambiguously.

## Per-Layer Context Schemas

Each layer (L1–L10) has a strongly-typed context message:

| Layer | Context Message | Key Fields |
|-------|-----------------|-----------|
| L1 | ContextL1 | GPU util, memory, thermal |
| L2 | ContextL2 | Model version, quantization |
| L3 | ContextL3 | Token count, special tokens |
| L4 | ContextL4 | Attention, KV cache, latency |
| L5 | ContextL5 | Token count, logprobs, finish reason |
| L6 | ContextL6 | Classifier scores, filter triggers |
| L7 | ContextL7 | Query, retrieval scores, latency |
| L8 | ContextL8 | Tool name, arguments, results |
| L9 | ContextL9 | Auth, rate limit, routing |
| L10 | ContextL10 | Session, user, message |

See `signal.proto` for full definitions.

## Wire Protocol Guarantees

### Immutability

Once a signal is written to ClickHouse, the schema contract is:
- Every field that exists in the schema must be queryable
- Field numbers are immutable (identify columns in ClickHouse)
- New SDKs and old SDKs can read the same signals (backward compatibility)

### Field Numbers

Field numbers identify the wire format representation. Reusing a field number breaks all deployed SDKs that still reference the old number.

**Example: Why field reuse is catastrophic**

```protobuf
// v1.0.0
message Foo {
  int32 old_field = 1;
}

// v2.0.0 (WRONG)
message Foo {
  string new_field = 1;  // Reuses field 1
  reserved 2;  // Old SDK still reads field 1 as int32
}
```

Old SDK sees `new_field` as an int32 → silently reads garbage.

**Correct approach:**

```protobuf
// v1.0.0
message Foo {
  int32 old_field = 1;
}

// v2.0.0 (CORRECT)
message Foo {
  int32 old_field = 1 [deprecated=true];
  string new_field = 2;  // New field number
}
```

Old SDK ignores `new_field`; new SDK reads both. Zero corruption.

## Generated Stubs

Run `buf generate` to produce:

```
proto/gen/
├── go/argus/v1/
│   ├── signal.pb.go
│   ├── signal_grpc.pb.go
│   ├── service.pb.go
│   └── service_grpc.pb.go
├── python/argus/v1/
│   ├── signal_pb2.py
│   ├── signal_pb2_grpc.py
│   ├── service_pb2.py
│   └── service_pb2_grpc.py
└── typescript/argus/v1/
    ├── signal.ts
    ├── service.ts
    └── ...
```

**All stubs are checked into git.** This ensures:
- Reproducible builds (stubs don't change unless `.proto` files change)
- CI can verify stubs compile
- SDKs can reference pinned stub versions
- No runtime code generation

## CI/CD

GitHub Actions workflow (`.github/workflows/protobuf.yaml`) runs on every PR:

1. **Format Check**: `buf format --diff` ensures consistent style
2. **Lint Check**: `buf lint` enforces naming, comment standards
3. **Breaking Check**: `buf breaking --against main` prevents schema drift
4. **Generation**: `buf generate` produces stubs
5. **Verification**: Go, Python, and TypeScript stubs compile

PRs that fail any check are blocked from merging.

## References

- [Protobuf Language Guide](https://protobuf.dev/reference/protobuf-language/)
- [Buf Documentation](https://buf.build/docs)
- [gRPC in Go](https://grpc.io/docs/languages/go/)
- [Schema Evolution Best Practices](../SCHEMA_EVOLUTION.md)
