# Argus Connector Package

High-level abstraction for emitting signals through configurable backends.
Wraps raw signal emission with lifecycle management, batching, error handling, and routing.

## Overview

Instead of directly using the low-level SDK (`emit_signal`), use **Sensors** and **Connectors**:

```python
from sdk.connector import Sensor, ConnectorType, Layer, Severity

# Initialize sensor (one per app)
sensor = Sensor(
    connector_type=ConnectorType.BUFFER,  # Use batching
    config={
        "base_url": "http://localhost:8080",
        "app_id": "my-app",
        "max_batch_size": 100,
        "flush_interval_seconds": 5.0,
    }
)

# Emit signals (clean, simple API)
await sensor.emit(
    layer=Layer.L5_OUTPUT_DECODING,
    category="inference.completion",
    severity=Severity.INFO,
    context={"tokens": 150, "latency_ms": 250},
    duration_ms=125.5
)

# Cleanup
await sensor.close()
```

## Connector Types

### 1. **DirectConnector** (Default)
Direct HTTP calls to Argus for each signal.

**Use when:**
- Low latency required
- Signals are infrequent
- Guaranteed delivery important

```python
sensor = Sensor(connector_type=ConnectorType.DIRECT, config={
    "base_url": "http://localhost:8080",
    "app_id": "my-app",
})
```

### 2. **BufferedConnector** (Recommended)
Batches signals and flushes periodically.

**Use when:**
- High volume of signals
- Network efficiency important
- Can tolerate brief delivery delay (5 seconds default)

```python
sensor = Sensor(connector_type=ConnectorType.BUFFER, config={
    "base_url": "http://localhost:8080",
    "app_id": "my-app",
    "max_batch_size": 100,           # Flush when 100 signals queued
    "flush_interval_seconds": 5.0,   # Or every 5 seconds
})
```

### 3. **NoOpConnector** (Testing)
No-op connector that logs but doesn't send.

```python
sensor = Sensor(connector_type=ConnectorType.NOOP)
```

### 4. **OTLPConnector** (Future)
Native OpenTelemetry Protocol export.

```python
# Coming soon - will export to /v1/traces directly
sensor = Sensor(connector_type=ConnectorType.OTLP, config={
    "endpoint": "http://localhost:8080",
})
```

## Usage Patterns

### Pattern 1: Context Manager (Recommended)

```python
async with Sensor(config={"app_id": "my-app"}) as sensor:
    # Emit signals
    await sensor.emit(
        layer=Layer.L5_OUTPUT_DECODING,
        category="inference",
    )
    # Auto-cleanup on exit
```

### Pattern 2: Manual Lifecycle

```python
sensor = Sensor(config={"app_id": "my-app"})

try:
    await sensor.emit(
        layer=Layer.L5_OUTPUT_DECODING,
        category="inference",
    )
finally:
    await sensor.close()
```

### Pattern 3: Decorator

```python
sensor = Sensor(config={"app_id": "my-app"})

@observe(sensor, Layer.L5_OUTPUT_DECODING, "inference.ollama")
async def call_ollama(prompt: str) -> str:
    # Duration captured automatically
    # Errors emitted as HIGH severity
    return await ollama.generate(prompt)

# Use it
result = await call_ollama("Hello")
```

### Pattern 4: Trace Correlation

```python
# Set trace ID from request
sensor.set_trace_id(request.headers.get("x-trace-id", str(uuid4())))

# All subsequent signals include this trace ID
await sensor.emit(layer=Layer.L5_OUTPUT_DECODING, category="step1")
await sensor.emit(layer=Layer.L7_RAG_RETRIEVAL, category="step2")
await sensor.emit(layer=Layer.L8_AGENTS, category="step3")
```

## API Reference

### Sensor Class

```python
class Sensor:
    def __init__(
        self,
        connector_type: ConnectorType = ConnectorType.DIRECT,
        config: Optional[Dict[str, Any]] = None,
    )

    async def emit(
        self,
        layer: Layer,
        category: str,
        severity: Severity = Severity.INFO,
        context: Optional[Dict[str, Any]] = None,
        duration_ms: Optional[float] = None,
    ) -> bool

    async def emit_error(
        self,
        layer: Layer,
        category: str,
        error: Exception,
        duration_ms: Optional[float] = None,
    ) -> bool

    def set_trace_id(self, trace_id: str)

    async def close()

    # Context manager
    async with Sensor(...) as sensor:
        await sensor.emit(...)
```

### Decorator

```python
@observe(
    sensor: Sensor,
    layer: Layer,
    category: str,
    severity: Severity = Severity.INFO,
)
async def my_function():
    # Duration and errors captured
    pass
```

## Configuration

### Common Config

```python
config = {
    # Backend connection
    "base_url": "http://localhost:8080",
    "app_id": "my-app",
    "app_version": "1.0.0",
    "environment": "production",
    "timeout": 30.0,

    # Buffering (BufferedConnector only)
    "max_batch_size": 100,
    "flush_interval_seconds": 5.0,
}
```

## Examples

### Example 1: Instrument Ollama Wrapper

```python
from sdk.connector import Sensor, ConnectorType, Layer, Severity
import aiohttp
import time

class InstrumentedOllama:
    def __init__(self, base_url="http://localhost:11434"):
        self.base_url = base_url
        self.sensor = Sensor(
            connector_type=ConnectorType.BUFFER,
            config={
                "base_url": "http://localhost:8080",
                "app_id": "ollama-instrumented",
            }
        )

    async def generate(self, model: str, prompt: str) -> str:
        """Generate response with signal emission"""
        start = time.time()
        try:
            async with aiohttp.ClientSession() as session:
                async with session.post(
                    f"{self.base_url}/api/generate",
                    json={"model": model, "prompt": prompt},
                ) as resp:
                    result = await resp.text()

            duration_ms = (time.time() - start) * 1000
            await self.sensor.emit(
                layer=Layer.L5_OUTPUT_DECODING,
                category="inference.ollama",
                severity=Severity.INFO,
                context={
                    "model": model,
                    "prompt_length": len(prompt),
                    "response_length": len(result),
                },
                duration_ms=duration_ms,
            )
            return result

        except Exception as e:
            duration_ms = (time.time() - start) * 1000
            await self.sensor.emit_error(
                layer=Layer.L5_OUTPUT_DECODING,
                category="inference.ollama",
                error=e,
                duration_ms=duration_ms,
            )
            raise

    async def close(self):
        await self.sensor.close()
```

### Example 2: FastAPI Middleware

```python
from fastapi import FastAPI, Request
from sdk.connector import Sensor, Layer, Severity
import time
import uuid

app = FastAPI()
sensor = Sensor(config={"app_id": "fastapi-app"})

@app.middleware("http")
async def sensor_middleware(request: Request, call_next):
    # Create trace ID if not present
    trace_id = request.headers.get("x-trace-id") or str(uuid.uuid4())
    sensor.set_trace_id(trace_id)

    start = time.time()
    try:
        response = await call_next(request)
        duration_ms = (time.time() - start) * 1000

        # Emit signal for successful request
        await sensor.emit(
            layer=Layer.L10_APPLICATION,
            category=f"http.{request.method.lower()}",
            severity=Severity.INFO if response.status_code < 400 else Severity.HIGH,
            context={
                "path": request.url.path,
                "status": response.status_code,
                "method": request.method,
            },
            duration_ms=duration_ms,
        )
        return response

    except Exception as e:
        duration_ms = (time.time() - start) * 1000
        await sensor.emit_error(
            layer=Layer.L10_APPLICATION,
            category="http.error",
            error=e,
            duration_ms=duration_ms,
        )
        raise
```

### Example 3: RAG Pipeline with Trace Correlation

```python
from sdk.connector import Sensor, Layer

async def rag_pipeline(query: str, trace_id: str):
    sensor = Sensor(config={"app_id": "rag-app"})
    sensor.set_trace_id(trace_id)

    try:
        # Step 1: Retrieval
        await sensor.emit(
            layer=Layer.L7_RAG_RETRIEVAL,
            category="retrieval.search",
            context={"query": query},
        )
        documents = await retrieve_documents(query)

        # Step 2: Ranking
        await sensor.emit(
            layer=Layer.L7_RAG_RETRIEVAL,
            category="retrieval.rerank",
            context={"documents_count": len(documents)},
        )
        top_docs = await rerank_documents(documents)

        # Step 3: Generation
        await sensor.emit(
            layer=Layer.L5_OUTPUT_DECODING,
            category="inference.generation",
            context={"context_length": sum(len(d) for d in top_docs)},
        )
        answer = await generate_answer(top_docs, query)

        return answer

    finally:
        await sensor.close()
```

## Migration from Old SDK

### Before (Old Direct SDK)

```python
from sdk.client import ArgusClient, Layer

client = ArgusClient(base_url="http://localhost:8080")
success = await client.emit_signal(
    layer=Layer.L5_OUTPUT_DECODING,
    category="test",
)
```

### After (Connector)

```python
from sdk.connector import Sensor, ConnectorType, Layer

sensor = Sensor(
    connector_type=ConnectorType.DIRECT,
    config={"base_url": "http://localhost:8080"}
)
success = await sensor.emit(
    layer=Layer.L5_OUTPUT_DECODING,
    category="test",
)
await sensor.close()
```

## Performance Tips

1. **Use BufferedConnector** for high-volume apps (>100 signals/sec)
2. **Set appropriate batch size** based on your signal volume
3. **Use async patterns** - never block on signal emission
4. **Fail-open** - signals are non-critical, app should continue if Argus down
5. **Monitor** - check drop counters via metrics endpoint

## Testing

```bash
# Test with no-op connector
sensor = Sensor(connector_type=ConnectorType.NOOP)
await sensor.emit(...)  # Won't send anywhere

# Test with logging
import logging
logging.basicConfig(level=logging.DEBUG)
# See all signal emissions in logs
```

## Architecture

```
Application Code
      ↓
   @observe or manual emit()
      ↓
   Sensor (high-level API)
      ↓
   Connector (pluggable backend)
      ├─ DirectConnector (HTTP per signal)
      ├─ BufferedConnector (batched HTTP)
      ├─ OTLPConnector (OTLP protocol)
      └─ NoOpConnector (testing)
      ↓
   Argus Backend
      ↓
   ClickHouse (signals storage)
```

## Next Steps

- [ ] Implement OTLP native connector
- [ ] Kafka connector for high-volume setups
- [ ] Metrics exporter (drop counter, latency histogram)
- [ ] Distributed tracing integration
- [ ] Language-specific agents (Go, JS, Java)
