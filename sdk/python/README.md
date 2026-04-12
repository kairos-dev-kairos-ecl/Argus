# Argus SDK for Python

Production-grade signal emission SDK for Python applications.

## Features

- **Minimal Overhead**: <5ms p99 overhead per signal
- **Fail-Open**: Continues if Argus unreachable (drop counter)
- **Decorator Pattern**: Simple `@observe()` for automatic instrumentation
- **Async-First**: Built on asyncio
- **Trace Correlation**: Automatic trace ID propagation
- **10-Layer Coverage**: Full LLM system taxonomy support

## Installation

```bash
pip install argus-sdk
```

## Quick Start

```python
import asyncio
from argus_sdk import ArgusClient, Layer, observe, Severity

# Initialize client
client = ArgusClient(
    base_url="http://localhost:8080",
    app_id="my-app",
    app_version="1.0.0",
)

# Use decorator for automatic instrumentation
@observe(
    layer=Layer.L5_OUTPUT_DECODING,
    category="inference.completion",
    client=client,
)
async def generate_response(prompt: str) -> str:
    return "Generated response"

# Or emit signals manually
async def main():
    async with client as client:
        success = await client.emit_signal(
            layer=Layer.L5_OUTPUT_DECODING,
            category="inference.completion",
            severity=Severity.HIGH,
            context={
                "output_tokens": 150,
                "input_tokens": 50,
                "finish_reason": "stop",
            }
        )
        print(f"Signal emitted: {success}")

asyncio.run(main())
```

## Usage Patterns

### Async Function with Decorator

```python
from argus_sdk import observe, Layer, Severity

@observe(
    layer=Layer.L7_RAG_RETRIEVAL,
    category="retrieval.search",
    severity=Severity.INFO,
    client=argus_client,
)
async def search_documents(query: str) -> List[str]:
    # Duration is measured automatically
    # Exceptions are caught and emitted as HIGH severity
    results = await db.search(query)
    return results
```

### Manual Signal Emission

```python
async with ArgusClient() as client:
    await client.emit_signal(
        layer=Layer.L8_AGENTS,
        category="tool_call.execution",
        severity=Severity.INFO,
        context={
            "tool_name": "web_search",
            "tool_result": "...",
            "tool_latency_ms": 250,
        },
        duration_ms=125,
    )
```

### With Trace Correlation

```python
# Set trace ID from upstream
client.set_trace_id(request.headers.get("x-trace-id"))

# All subsequent signals use this trace ID
await client.emit_signal(layer=Layer.L5_OUTPUT_DECODING, ...)
```

## Configuration

```python
client = ArgusClient(
    base_url="http://localhost:8080",  # Argus endpoint
    app_id="my-app",                   # Application ID
    app_version="1.0.0",               # Semantic version
    sdk_version="0.1.0",               # (auto-detected)
    environment="production",          # dev/staging/prod
    timeout=30.0,                      # HTTP timeout in seconds
)
```

## Buffering

For high-volume applications, use buffering:

```python
from argus_sdk import SignalBuffer

buffer = SignalBuffer(
    max_size=1000,              # Max signals before drop
    flush_interval_seconds=1.0, # Auto-flush interval
)

# Signals are buffered and flushed periodically
await buffer.add(signal)

# Monitor drops
drops = buffer.get_drop_counter()
```

## Fail-Open Behavior

The SDK gracefully handles Argus unavailability:

```python
# If Argus is down:
# 1. Signal emission returns False (not exception)
# 2. Drop counter increments
# 3. Application continues normally

success = await client.emit_signal(...)  # False if Argus down
print(f"Signal emitted: {success}")  # False, but no exception
```

## Supported Layers

| Layer | Name | Example Categories |
|-------|------|-------------------|
| L1 | Hardware | memory, power |
| L2 | Model Weights | loading, caching |
| L3 | Tokenizer | encode, decode |
| L4 | Transformer | attention, mha |
| L5 | Output Decoding | completion, streaming |
| L6 | Safety | check, violation |
| L7 | RAG Retrieval | search, rerank |
| L8 | Agents | planning, tool_call |
| L9 | API Gateway | http, grpc |
| L10 | Application | business_logic, user_action |

## Performance

Benchmark your application:

```python
import time

start = time.time()
await client.emit_signal(layer=Layer.L5_OUTPUT_DECODING, category="test")
overhead_ms = (time.time() - start) * 1000
print(f"Overhead: {overhead_ms:.2f}ms")
```

**Target: <5ms p99 overhead**

## Testing

Run tests:

```bash
pip install pytest pytest-asyncio
pytest sdk/tests/ -v
```

## API Reference

### ArgusClient

```python
class ArgusClient:
    def __init__(
        self,
        base_url: str = "http://localhost:8080",
        app_id: str = "test-app",
        app_version: str = "0.1.0",
        sdk_version: str = "0.1.0",
        environment: str = "test",
        timeout: float = 30.0,
    ) -> None

    async def emit_signal(
        self,
        layer: Layer,
        category: str,
        severity: Severity = Severity.INFO,
        context: Optional[Dict[str, Any]] = None,
        duration_ms: Optional[float] = None,
        trace_id: Optional[str] = None,
        parent_span_id: Optional[str] = None,
    ) -> bool

    def set_trace_id(self, trace_id: str) -> None
    def get_trace_id(self) -> str
```

### @observe Decorator

```python
@observe(
    layer: Layer,
    category: str,
    severity: Severity = Severity.INFO,
    client: Optional[ArgusClient] = None,
    buffer_enabled: bool = True,
    max_buffer_size: int = 100,
)
```

## Examples

See `apps/` directory for complete examples:

- `rag-app/`: RAG pipeline (retrieval + inference)
- `agent-app/`: Agentic decision-making with tools
- `chatbot-app/`: Multi-turn conversation

## Documentation

- [SDK Guide](../../docs/SDK_GUIDE.md) - Comprehensive guide
- [Reference Apps](../../docs/REFERENCE_APPS.md) - Example implementations
- [OTEL Integration](../../docs/OTEL_INTEGRATION.md) - OpenTelemetry bridge

## Support

- Issues: https://github.com/argusxdr/argus/issues
- Docs: https://docs.argusxdr.io
- Chat: https://discord.gg/argusxdr (community)

## License

Apache 2.0
