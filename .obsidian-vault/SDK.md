# SDK

> Language-specific clients for emitting signals. Python + TypeScript.

## SDK Design

All SDKs follow the same pattern:
1. Initialize `ArgusClient` with endpoint + API key
2. Use `SignalBuilder` (builder pattern) to construct signals
3. Call `emit_signal()` to POST to `/v1/signals`
4. Optionally use decorators/hooks for auto-instrumentation

---

## Python SDK

Location: `sdk/python/` + `sdk/client.py` + `sdk/signal_builder.py` + `sdk/decorator.py`

### ArgusClient

```python
class ArgusClient:
    def __init__(self, endpoint: str, api_key: str, app_id: str):
        self.endpoint = endpoint  # "http://localhost:8080"
        self.api_key = api_key    # from GET /api/v1/apps/{id}/key
        self.app_id = app_id
        self.session = httpx.AsyncClient()
        self.trace_id = str(uuid.uuid4())  # context: one trace per logical operation
    
    async def emit_signal(self, signal: ArgusSignal) -> IngestResponse:
        """Emit protobuf signal"""
        # Auto-set: signal.source.app_id, ingested_at, trace_id (if not set)
        return await self.session.post(
            f"{self.endpoint}/v1/signals",
            content=signal.SerializeToString(),
            headers={
                'X-Argus-Key': self.api_key,
                'Content-Type': 'application/protobuf'
            }
        )
    
    async def emit_signal_json(self, signal_dict: dict) -> IngestResponse:
        """Emit JSON signal (auto-converts to protobuf)"""
        signal = ArgusSignal(**signal_dict)
        return await self.emit_signal(signal)
```

Uses: `httpx` (async HTTP), `protobuf` (serialization), `uuid`, `psutil` (metrics)

Generated proto bindings: `sdk/gen/argus/v1/signal_pb2.py`

### SignalBuilder

```python
class SignalBuilder:
    def __init__(self, client: ArgusClient, layer: Layer):
        self.client = client
        self.signal = ArgusSignal(
            signal_id=str(ULID()),
            trace_id=client.trace_id,
            source=Source(app_id=client.app_id, app_version="1.0", ...),
            layer=layer
        )
    
    def category(self, cat: str):
        self.signal.category = cat
        return self
    
    def severity(self, sev: Severity):
        self.signal.severity = sev
        return self
    
    def with_l1_context(self, cpu_percent, memory_mb):
        self.signal.context.l1 = ContextL1(
            cpu_percent=cpu_percent,
            memory_used_mb=memory_mb
        )
        return self
    
    # ... with_l2_context, with_l3_context, ... with_l10_context
    
    async def emit(self):
        return await self.client.emit_signal(self.signal)
```

Usage:
```python
async with ArgusClient("http://localhost:8080", "key_xyz", "my-app") as client:
    await (SignalBuilder(client, Layer.L1_HARDWARE)
        .category("infra.cpu")
        .severity(Severity.MEDIUM)
        .with_l1_context(cpu_percent=85.0, memory_mb=6500)
        .emit())
```

### Decorator

```python
@argus_trace(client=client, layer=Layer.L5_OUTPUT)
def generate_response(prompt: str) -> str:
    """Auto-instrumented LLM call"""
    # Decorator captures:
    # - execution time (duration_ms)
    # - CPU/memory before/after (L1 context)
    # - exception details (if raised)
    output = llm.generate(prompt)
    # Decorator also captures:
    # - output tokens, finish reason, logprobs (L5 context)
    return output
```

### Buffer

For high-frequency signals, batch them:

```python
buffer = SignalBuffer(client, batch_size=100, flush_interval=2.0)
for item in high_frequency_stream:
    signal = build_signal(item)
    buffer.add(signal)  # non-blocking; auto-flushes at batch_size or interval
```

---

## TypeScript SDK

Location: `sdk/typescript/src/` + `package.json`

### ArgusClient

```typescript
class ArgusClient {
  private endpoint: string;
  private apiKey: string;
  private appId: string;
  private traceId: string;

  constructor(endpoint: string, apiKey: string, appId: string) {
    this.endpoint = endpoint;
    this.apiKey = apiKey;
    this.appId = appId;
    this.traceId = crypto.randomUUID();
  }

  async emitSignal(signal: ArgusSignal): Promise<IngestResponse> {
    const response = await fetch(`${this.endpoint}/v1/signals`, {
      method: 'POST',
      headers: {
        'X-Argus-Key': this.apiKey,
        'Content-Type': 'application/json'
      },
      body: JSON.stringify(signal)
    });
    return response.json();
  }
}
```

Uses: native `fetch`, `crypto.randomUUID()`, protocol buffers (via `@types/google-protobuf`)

### SignalBuilder

```typescript
class SignalBuilder {
  private signal: ArgusSignal;

  constructor(client: ArgusClient, layer: Layer) {
    this.signal = {
      signal_id: ulid(),
      trace_id: client.traceId,
      layer,
      source: { app_id: client.appId, app_version: '1.0' }
    };
  }

  category(cat: string): this {
    this.signal.category = cat;
    return this;
  }

  withL1Context(cpuPercent: number, memoryMb: number): this {
    this.signal.context = {
      l1: { cpu_percent: cpuPercent, memory_used_mb: memoryMb }
    };
    return this;
  }

  async emit(): Promise<IngestResponse> {
    return this.client.emitSignal(this.signal);
  }
}
```

### Testing

Jest tests: `sdk/typescript/tests/`
- Client initialization
- Signal emission (mock endpoint)
- Builder chaining
- Error handling (network, auth, validation)

---

## Generated Protobuf Bindings

Both SDKs generate stubs from proto files:

### Python
```bash
python -m grpc_tools.protoc \
  -I proto/ \
  --python_out=sdk/gen/argus/v1/ \
  --grpc_python_out=sdk/gen/argus/v1/ \
  proto/argus/v1/signal.proto
```

Output: `sdk/gen/argus/v1/signal_pb2.py`, `signal_pb2_grpc.py`

### TypeScript
```bash
protoc \
  -I proto/ \
  --ts_out=sdk/typescript/src/gen/ \
  proto/argus/v1/signal.proto
```

Output: `sdk/typescript/src/gen/argus/v1/signal.ts`

---

## Test Harness (E2E Integration)

Location: `test_harness/` uses both SDKs to validate full pipeline.

### Python Test Harness
`test_harness/qwen_llama_api.py`:
- Instantiates `ArgusClient` + `LlamaCppClient` (llama.cpp HTTP client)
- Runs 3 LLM inference scenarios
- Emits all 10 layers per inference (L1 hardware, L2 model, L3 tokenizer, L5 decoding, etc.)
- Then calls `validate_signals.py` to verify signals were ingested

### Validation Script
`test_harness/validate_signals.py`:
- Queries `GET /v1/signals` with filters (app_id, layer)
- Verifies signal count per layer
- Checks schema compliance (required fields present)
- Validates trace correlation (related_signals populated)
- Outputs `validation_results.json` with pass/fail

---

## SDK Patterns

### Trace Context
```python
client = ArgusClient(..., app_id="my-app")
# All signals from this client share client.trace_id
# Use for grouping signals from one logical operation
for sub_task in tasks:
    await (SignalBuilder(client, Layer.L10_APPLICATION)
        .category("app.subtask")
        .emit())
```

### Span Hierarchy
```python
# Parent span
parent_signal = await (SignalBuilder(client, layer)
    .category("op.main")
    .emit())

# Child span
child_signal = await (SignalBuilder(client, layer)
    .category("op.child")
    .with_parent_span_id(parent_signal.span_id)
    .emit())
```

### Error Handling
```python
try:
    await (SignalBuilder(client, Layer.L5_OUTPUT)
        .with_l5_context(finish_reason="length")
        .emit())
except httpx.HTTPStatusError as e:
    if e.response.status_code == 429:
        print("Queue full — backoff")
    elif e.response.status_code == 401:
        print("API key revoked")
```

---

## File Map

| File | Component |
|------|-----------|
| `sdk/client.py` | ArgusClient (async httpx) |
| `sdk/signal_builder.py` | SignalBuilder (fluent API) |
| `sdk/decorator.py` | Python decorators for auto-instrumentation |
| `sdk/buffer.py` | SignalBuffer (batching) |
| `sdk/connector.py` | Connector interface |
| `sdk/gen/argus/v1/signal_pb2.py` | Generated Python proto bindings |
| `sdk/typescript/src/client.ts` | ArgusClient (fetch) |
| `sdk/typescript/src/signal_builder.ts` | SignalBuilder |
| `sdk/typescript/tests/` | Jest test suite |
| `test_harness/qwen_llama_api.py` | E2E Python test harness |
| `test_harness/validate_signals.py` | Signal validation suite |
