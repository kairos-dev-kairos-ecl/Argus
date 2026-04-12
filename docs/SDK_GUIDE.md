# Argus SDK Guide

Comprehensive guide for instrumenting applications with the Argus SDK.

## Overview

The Argus SDK provides signal emission capabilities for Python and TypeScript/Node.js applications. Signals are lightweight, asynchronous events that capture operational data across all 10 layers of an LLM system.

**Key Features:**
- Minimal overhead (<5ms p99 overhead)
- Fail-open behavior (drops signals, doesn't crash)
- Automatic trace correlation
- Layer and category-based organization
- Protobuf wire format for efficiency

## Installation

### Python

```bash
pip install argus-sdk
```

### TypeScript/Node.js

```bash
npm install @argusxdr/sdk
```

## Quick Start

### Python

```python
import asyncio
from argus_sdk import ArgusClient, Layer, Severity, observe

# Initialize client
argus_client = ArgusClient(
    base_url="http://localhost:8080",
    app_id="my-app",
    app_version="1.0.0",
)

# Use as context manager
async def main():
    async with argus_client as client:
        # Emit a signal directly
        await client.emit_signal(
            layer=Layer.L5_OUTPUT_DECODING,
            category="inference.completion",
            severity=Severity.INFO,
            context={
                "output_tokens": 150,
                "input_tokens": 50,
                "finish_reason": "stop",
            }
        )

# Or use the decorator
@observe(
    layer=Layer.L5_OUTPUT_DECODING,
    category="inference.generation",
    client=argus_client,
)
async def generate_response(prompt: str) -> str:
    # Your LLM inference logic
    return "Generated response"

asyncio.run(main())
```

### TypeScript/Node.js

```typescript
import { ArgusClient, Layer, Severity, argusMiddleware } from '@argusxdr/sdk';
import express from 'express';

const app = express();
const client = new ArgusClient(
  'http://localhost:8080',
  'my-app',
  '1.0.0'
);

await client.initialize();

// Use as middleware
app.use(argusMiddleware({ client, layer: Layer.L9_API_GATEWAY }));

// Emit signals manually
app.post('/api/inference', async (req, res) => {
  const success = await client.emitSignal(
    Layer.L5_OUTPUT_DECODING,
    'inference.completion',
    Severity.INFO,
    {
      output_tokens: 150,
      input_tokens: 50,
    }
  );
  res.json({ success });
});

app.listen(3000);
```

## Configuration

### Python Client Configuration

```python
from argus_sdk import ArgusClient

client = ArgusClient(
    base_url="http://localhost:8080",      # Argus endpoint
    app_id="my-app",                       # Your application ID
    app_version="1.0.0",                   # Semantic version
    sdk_version="0.1.0",                   # SDK version (auto)
    environment="production",               # Environment
    timeout=30.0,                          # HTTP timeout (seconds)
)
```

### TypeScript/Node.js Configuration

```typescript
import { ArgusClient, ClientConfig } from '@argusxdr/sdk';

const config: ClientConfig = {
  baseUrl: 'http://localhost:8080',
  appId: 'my-app',
  appVersion: '1.0.0',
  sdkVersion: '0.1.0',
  environment: 'production',
  timeout: 30000,  // milliseconds
  bufferSize: 100,
  flushInterval: 1000,  // milliseconds
};

const client = new ArgusClient(
  config.baseUrl,
  config.appId,
  config.appVersion,
);
```

## Layers and Categories

### 10-Layer LLM System Taxonomy

| Layer | Name | Purpose | Example Categories |
|-------|------|---------|-------------------|
| L1 | Hardware | GPU/TPU operations | memory.allocation, power.usage |
| L2 | Model Weights | Model loading/caching | weights.loading, cache.hit |
| L3 | Tokenizer | Tokenization | tokenization.encode, tokenization.decode |
| L4 | Transformer | Core neural computation | attention.forward, mha.compute |
| L5 | Output Decoding | Token generation | inference.completion, streaming.chunk |
| L6 | Safety | Safety filters | safety.check, jailbreak.detected |
| L7 | RAG Retrieval | Document retrieval | retrieval.search, reranking.applied |
| L8 | Agents | Agentic behavior | agent.planning, tool.call |
| L9 | API Gateway | API/network layer | http.request, grpc.call |
| L10 | Application | User-facing logic | business.logic, user.action |

### Common Categories

**Retrieval (L7):**
- `retrieval.search` - Vector/keyword search
- `retrieval.rerank` - Re-ranking results
- `retrieval.chunk_inject` - Injecting chunks

**Inference (L5, L8):**
- `inference.completion` - LLM completion
- `inference.streaming` - Streaming tokens
- `inference.chat_response` - Chat response

**Agent Actions (L8):**
- `agent.planning` - Creating a plan
- `agent.tool_selection` - Selecting tools
- `agent.tool_call` - Invoking tools

**Safety (L6):**
- `safety.check` - Safety evaluation
- `safety.violation` - Policy violation
- `safety.moderation` - Content moderation

## Decorator Pattern (Python)

The `@observe()` decorator automatically emits signals for function execution:

```python
from argus_sdk import observe, Layer

# Async function
@observe(
    layer=Layer.L7_RAG_RETRIEVAL,
    category="retrieval.search",
    client=argus_client,
)
async def search_documents(query: str) -> List[str]:
    # Function automatically emits signal on entry/exit
    # Duration is measured automatically
    # Exceptions are caught and logged
    return results

# Sync function
@observe(
    layer=Layer.L10_APPLICATION,
    category="business.logic",
    client=argus_client,
)
def process_data(data: dict) -> dict:
    # Sync functions emit signals fire-and-forget
    return processed_data
```

**Features:**
- Duration measurement in milliseconds
- Automatic exception handling
- Fail-open (continues on error)
- Trace ID propagation
- Custom severity levels

## Middleware Pattern (TypeScript)

Express middleware automatically instruments HTTP requests:

```typescript
import { argusMiddleware, Layer } from '@argusxdr/sdk';

app.use(argusMiddleware({
  client: argusClient,
  layer: Layer.L9_API_GATEWAY,
  category: 'http.request',
  includRequestBody: true,   // Include request body in signal
  includeResponseBody: true, // Include response body in signal
  excludePaths: ['/health', '/metrics'],
}));
```

**Captured Metrics:**
- HTTP method and path
- Response status code
- Request/response latency
- Request/response bodies (optional)

## Error Handling

### Python

```python
from argus_sdk import ArgusClient, Layer

async def main():
    try:
        async with ArgusClient() as client:
            # If Argus is unreachable, signals are dropped silently
            # (fail-open behavior)
            success = await client.emit_signal(
                layer=Layer.L5_OUTPUT_DECODING,
                category="inference.completion",
            )
            print(f"Signal emitted: {success}")
    except Exception as e:
        # Argus unreachable is not an exception in fail-open mode
        print(f"Error: {e}")
```

### TypeScript

```typescript
try {
  await client.emitSignal(
    Layer.L5_OUTPUT_DECODING,
    'inference.completion',
    Severity.INFO,
  );
} catch (error) {
  // Catches only client initialization errors
  // Unreachable Argus returns false, doesn't throw
  console.error('Client error:', error);
}
```

## Fail-Open Behavior

The SDK is designed to **fail open**: if Argus is unreachable, signals are dropped but your application continues.

### Drop Counter

When signals are dropped, the SDK maintains a drop counter. Periodically, the drop counter itself is emitted as a special signal:

```python
from argus_sdk import SignalBuffer

buffer = SignalBuffer(max_size=100)

# If buffer fills, signals are dropped
# Drop counter is tracked
drops = buffer.get_drop_counter()  # e.g., 5 signals dropped

# Emit drop counter signal when you reconnect
await buffer.emit_drop_counter_signal(emit_fn, ...)
```

### Configuration for Reliability

```python
# Increase buffer size for burst traffic
buffer = SignalBuffer(max_size=1000)

# Shorter flush interval for lower latency
buffer = SignalBuffer(flush_interval_seconds=0.1)
```

## Trace Correlation

Trace IDs link signals across your system:

```python
# Auto-generate trace ID
trace_id = client.get_trace_id()

# Set custom trace ID (e.g., from upstream service)
client.set_trace_id("external-trace-123")

# Emit signal with trace ID
await client.emit_signal(
    layer=Layer.L5_OUTPUT_DECODING,
    category="inference.completion",
    trace_id=trace_id,  # Optional, auto-generated if not provided
)
```

## Layer-Specific Context

Each layer supports layer-specific context fields:

```python
# L5 (Output Decoding) context
await client.emit_signal(
    layer=Layer.L5_OUTPUT_DECODING,
    category="inference.completion",
    context={
        "operation": 1,  # GENERATION
        "input_tokens": 50,
        "output_tokens": 150,
        "finish_reason": "stop",
        "temperature": 0.7,
        "top_p": 1.0,
    }
)

# L7 (RAG Retrieval) context
await client.emit_signal(
    layer=Layer.L7_RAG_RETRIEVAL,
    category="retrieval.search",
    context={
        "operation": 1,  # VECTOR_SEARCH
        "query_text": "What is Argus?",
        "results_count": 10,
        "embedding_model": "sentence-transformers/all-MiniLM-L6-v2",
        "vector_index": "production-index",
        "context_window_pct": 85.0,
    }
)

# L8 (Agents) context
await client.emit_signal(
    layer=Layer.L8_AGENTS,
    category="tool_call.execution",
    context={
        "operation": 1,  # TOOL_CALL
        "tool_name": "web_search",
        "tool_provider": "google",
        "tool_arguments": {
            "query": "search term",
            "max_results": 10,
        },
        "tool_result": "Result JSON",
        "tool_latency_ms": 123.4,
        "step_number": 1,
        "total_steps": 3,
    }
)
```

## Performance Tuning

### Reduce Overhead

1. **Increase emit batching:**
   ```python
   buffer = SignalBuffer(max_size=1000, flush_interval_seconds=2.0)
   ```

2. **Use sampling:**
   ```python
   import random
   if random.random() < 0.1:  # 10% sampling
       await client.emit_signal(...)
   ```

3. **Disable context capture:**
   ```python
   await client.emit_signal(
       layer=Layer.L5_OUTPUT_DECODING,
       category="inference.completion",
       # No context = faster
   )
   ```

### Monitor Overhead

```python
import time

# Measure overhead
start = time.time()
await client.emit_signal(layer=Layer.L5_OUTPUT_DECODING, category="test")
overhead_ms = (time.time() - start) * 1000
print(f"Signal overhead: {overhead_ms:.2f}ms")
```

## Troubleshooting

### Signals not appearing in Argus

1. **Check connection:**
   ```bash
   curl http://localhost:8080/health
   ```

2. **Check app_id:**
   - Verify `app_id` in your client matches what you're filtering for
   
3. **Check category spelling:**
   - Category must be lowercase with dots (e.g., `inference.completion`)

4. **Verify Argus is running:**
   - Open dashboard: http://localhost:3000

### High overhead

1. Check network latency to Argus
2. Increase flush interval
3. Reduce batch frequency or use sampling
4. Check CPU/memory usage

### Signals being dropped

1. Check drop counter via `/metrics`
2. Increase buffer size
3. Reduce signal emission rate
4. Check Argus ingestion capacity

## Next Steps

- [Reference Applications](./REFERENCE_APPS.md) - See example apps
- [OpenTelemetry Integration](./OTEL_INTEGRATION.md) - OTEL bridge
- [Performance Baseline](./PERFORMANCE_BASELINE.md) - Benchmark results
- [Argus Dashboard](http://localhost:3000) - View signals

## Support

For issues, bugs, or feature requests:
- GitHub Issues: https://github.com/argusxdr/argus
- Docs: https://docs.argusxdr.io
