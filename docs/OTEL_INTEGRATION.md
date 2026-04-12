# OpenTelemetry Integration Guide

Bridge OpenTelemetry instrumentation to Argus signals.

## Overview

The OpenTelemetry (OTEL) integration allows you to:

1. Instrument your app with standard OTEL APIs
2. Export spans to Argus OTLP receiver
3. Automatically convert spans to Argus signals
4. Leverage existing OTEL ecosystem

**Key Benefits:**
- Standards-based instrumentation
- Multi-vendor support (can switch backends)
- Rich semantic conventions
- Lower integration cost

## Architecture

```
Your Application (OTEL instrumented)
    ↓
OTEL SDK (span generation)
    ↓
OTLP Exporter (gRPC or HTTP)
    ↓
Argus OTLP Receiver (port 4317)
    ↓
OTLPToArgusConverter (span → signal)
    ↓
Argus Ingest Pipeline
    ↓
ClickHouse (queryable signals)
```

## Setup

### 1. Install OpenTelemetry

```bash
pip install opentelemetry-api opentelemetry-sdk
pip install opentelemetry-exporter-otlp-proto-grpc
pip install opentelemetry-instrumentation-fastapi
pip install opentelemetry-instrumentation-requests
```

For Node.js:
```bash
npm install @opentelemetry/api @opentelemetry/sdk-node
npm install @opentelemetry/exporter-trace-otlp-proto
npm install @opentelemetry/instrumentation-express
```

### 2. Configure OTEL Exporter

**Python:**
```python
from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.sdk.trace.export import BatchSpanProcessor

# Create exporter pointing to Argus OTLP receiver
exporter = OTLPSpanExporter(
    endpoint="localhost:4317",  # Argus OTLP receiver
    insecure=True,  # For local dev
)

# Configure tracer provider
trace.set_tracer_provider(TracerProvider())
trace.get_tracer_provider().add_span_processor(BatchSpanProcessor(exporter))

# Get tracer
tracer = trace.get_tracer(__name__)
```

**Node.js:**
```typescript
import { NodeSDK } from '@opentelemetry/sdk-node';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-proto';
import { getNodeAutoInstrumentations } from '@opentelemetry/auto-instrumentations-node';

const sdk = new NodeSDK({
  traceExporter: new OTLPTraceExporter({
    url: 'http://localhost:4317',  // Argus OTLP receiver
  }),
  instrumentations: [getNodeAutoInstrumentations()],
});

sdk.start();
```

### 3. Instrument Your Application

**Python - FastAPI:**
```python
from fastapi import FastAPI
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor

app = FastAPI()

# Auto-instrument FastAPI
FastAPIInstrumentor.instrument_app(app)

@app.post("/api/endpoint")
async def my_endpoint(request: MyRequest):
    return {"status": "ok"}
```

**Python - Manual Instrumentation:**
```python
from opentelemetry import trace

tracer = trace.get_tracer(__name__)

@app.post("/api/search")
async def search(query: str):
    with tracer.start_as_current_span("vector_search") as span:
        # Set attributes for layer mapping
        span.set_attribute("argus.layer", 7)  # L7_RAG_RETRIEVAL
        span.set_attribute("argus.category", "retrieval.vector_search")
        span.set_attribute("retrieval.query", query)
        span.set_attribute("retrieval.results_count", 10)
        span.set_attribute("service.name", "my-app")
        span.set_attribute("service.version", "1.0.0")
        
        # Your search logic
        results = await vector_db.search(query)
        
        span.set_attribute("retrieval.operation", 1)  # VECTOR_SEARCH
        span.set_attribute("retrieval.embedding_model", "sentence-transformers/all-MiniLM-L6-v2")
        
        return results
```

**Node.js - Express:**
```typescript
import express from 'express';
import { NodeSDK } from '@opentelemetry/sdk-node';
import { ExpressInstrumentation } from '@opentelemetry/instrumentation-express';

const app = express();

// Auto-instrument Express
const sdk = new NodeSDK({
  instrumentations: [new ExpressInstrumentation()],
});
sdk.start();

app.post('/api/search', async (req, res) => {
  const span = trace.getActiveSpan();
  
  if (span) {
    span.setAttributes({
      'argus.layer': 7,
      'argus.category': 'retrieval.vector_search',
      'retrieval.query': req.body.query,
      'service.name': 'my-app',
    });
  }
  
  const results = await vectorDb.search(req.body.query);
  res.json(results);
});
```

## Span Attributes Reference

### Required Attributes

For proper layer mapping, set at least one of:

```python
# Option 1: Explicit layer
span.set_attribute("argus.layer", 7)  # L7_RAG_RETRIEVAL

# Option 2: Span kind (auto-mapped)
# span.kind = SpanKind.CLIENT  # Maps to L9_API_GATEWAY
```

### Recommended Attributes

**All spans:**
```python
span.set_attribute("service.name", "my-app")
span.set_attribute("service.version", "1.0.0")
span.set_attribute("service.instance.id", "instance-1")
span.set_attribute("deployment.environment", "production")
span.set_attribute("argus.category", "retrieval.search")
```

### Layer-Specific Attributes

**L5 (Output Decoding) - LLM Inference:**
```python
span.set_attribute("argus.layer", 5)
span.set_attribute("llm.operation", 1)  # GENERATION
span.set_attribute("llm.input_tokens", 50)
span.set_attribute("llm.output_tokens", 150)
span.set_attribute("llm.finish_reason", "stop")
span.set_attribute("llm.temperature", 0.7)
span.set_attribute("llm.top_p", 1.0)
```

**L6 (Safety) - Safety Checks:**
```python
span.set_attribute("argus.layer", 6)
span.set_attribute("safety.check_type", "jailbreak_detection")
span.set_attribute("safety.violation_detected", False)
span.set_attribute("safety.score", 0.05)  # 0-1, higher = more unsafe
```

**L7 (RAG Retrieval) - Document Search:**
```python
span.set_attribute("argus.layer", 7)
span.set_attribute("retrieval.operation", 1)  # VECTOR_SEARCH
span.set_attribute("retrieval.query", "search query")
span.set_attribute("retrieval.results_count", 10)
span.set_attribute("retrieval.embedding_model", "all-MiniLM-L6-v2")
span.set_attribute("retrieval.vector_index", "prod-index-v1")
span.set_attribute("retrieval.context_window_pct", 85.0)
```

**L8 (Agents) - Tool Calling:**
```python
span.set_attribute("argus.layer", 8)
span.set_attribute("agent.operation", 1)  # TOOL_CALL
span.set_attribute("agent.tool_name", "web_search")
span.set_attribute("agent.tool_provider", "google")
span.set_attribute("agent.step_number", 1)
span.set_attribute("agent.total_steps", 3)

# Tool arguments as JSON
span.set_attribute("agent.tool_arguments", '{"query": "..."}')
```

**L9 (API Gateway) - HTTP Requests:**
```python
span.set_attribute("argus.layer", 9)
span.set_attribute("http.method", "POST")
span.set_attribute("http.url", "/api/endpoint")
span.set_attribute("http.status_code", 200)
span.set_attribute("http.response_content_length", 1024)
```

**L10 (Application) - Business Logic:**
```python
span.set_attribute("argus.layer", 10)
span.set_attribute("business.entity", "user")
span.set_attribute("business.operation", "purchase")
span.set_attribute("business.status", "completed")
```

## Examples

### RAG Pipeline

```python
from opentelemetry import trace
from opentelemetry.trace import SpanKind

tracer = trace.get_tracer(__name__)

async def rag_pipeline(query: str):
    # Step 1: Retrieve documents (L7)
    with tracer.start_as_current_span("retrieve_documents") as span:
        span.set_attribute("argus.layer", 7)
        span.set_attribute("argus.category", "retrieval.search")
        span.set_attribute("retrieval.operation", 1)  # VECTOR_SEARCH
        span.set_attribute("retrieval.query", query)
        
        documents = await vector_db.search(query)
        
        span.set_attribute("retrieval.results_count", len(documents))
        span.set_attribute("retrieval.embedding_model", "all-MiniLM-L6-v2")
    
    # Step 2: Generate answer (L5)
    with tracer.start_as_current_span("generate_answer") as span:
        span.set_attribute("argus.layer", 5)
        span.set_attribute("argus.category", "inference.completion")
        span.set_attribute("llm.operation", 1)  # GENERATION
        span.set_attribute("llm.input_tokens", 100)
        
        answer = await llm.generate(query, context=documents)
        
        span.set_attribute("llm.output_tokens", 150)
        span.set_attribute("llm.finish_reason", "stop")
    
    return answer
```

### Agent with Tools

```python
async def agent_loop(task: str):
    with tracer.start_as_current_span("agent_reasoning") as span:
        span.set_attribute("argus.layer", 8)
        span.set_attribute("argus.category", "agent.planning")
        
        plan = await planner.create_plan(task)
    
    for step_num, action in enumerate(plan):
        with tracer.start_as_current_span(f"tool_call_{step_num}") as span:
            span.set_attribute("argus.layer", 8)
            span.set_attribute("argus.category", "tool_call.execution")
            span.set_attribute("agent.operation", 1)  # TOOL_CALL
            span.set_attribute("agent.tool_name", action.tool)
            span.set_attribute("agent.step_number", step_num + 1)
            span.set_attribute("agent.total_steps", len(plan))
            span.set_attribute("agent.tool_arguments", json.dumps(action.args))
            
            result = await execute_tool(action.tool, action.args)
            
            span.set_attribute("agent.tool_result", str(result))
    
    return final_result
```

## Automatic Instrumentation

Use OTEL's automatic instrumentations for common frameworks:

**Python:**
```bash
pip install opentelemetry-auto-instrumentation[otlp]

# Run with auto-instrumentation
opentelemetry-instrument --exporter_otlp_endpoint=localhost:4317 python app.py
```

**Node.js:**
```bash
npm install @opentelemetry/auto-instrumentations-node

# In your app
import { getNodeAutoInstrumentations } from '@opentelemetry/auto-instrumentations-node';

const sdk = new NodeSDK({
  instrumentations: [getNodeAutoInstrumentations()],
});
```

## Span → Signal Conversion

How spans are converted to Argus signals:

**Input OTLP Span:**
```json
{
  "traceId": "abc123...",
  "spanId": "def456...",
  "name": "vector_search",
  "kind": 2,
  "startTimeUnixNano": 1704067200000000000,
  "endTimeUnixNano": 1704067200050000000,
  "attributes": [
    {"key": "argus.layer", "value": {"intValue": 7}},
    {"key": "retrieval.results_count", "value": {"intValue": 10}}
  ]
}
```

**Output ArgusSignal:**
```protobuf
signal_id: "1704067200000abc123def"
trace_id: "abc123..."
span_id: "def456..."
layer: 7  # From argus.layer attribute
category: "otel.vector_search"  # From span name
severity: 1  # INFO
duration_ms: 50.0
source: {
  app_id: "my-app"
  app_version: "1.0.0"
  sdk_version: "0.1.0"
  environment: "production"
}
context_l7: {
  results_count: 10  # From retrieval.results_count
}
```

## Best Practices

1. **Set `argus.layer` explicitly** - Don't rely on span kind guessing
2. **Use semantic conventions** - Follow OpenTelemetry spec
3. **Include service metadata** - `service.name`, `service.version`, `deployment.environment`
4. **Add context attributes** - Layer-specific details (token counts, results, etc.)
5. **Use span links** - For related spans (parent-child causality)
6. **Set span status** - OK or ERROR based on operation result

```python
from opentelemetry.trace import Status, StatusCode

try:
    result = await operation()
    span.set_status(Status(StatusCode.OK))
except Exception as e:
    span.record_exception(e)
    span.set_status(Status(StatusCode.ERROR, str(e)))
    raise
```

## Performance Impact

OTEL overhead is typically <1ms per span (excluding network):

- Span creation: <0.1ms
- Attribute setting: <0.01ms/attribute
- Exporting: 0-5ms (batched, asynchronous)

For <5ms total overhead, keep spans lightweight:
- Limit attributes to essentials
- Use batching (default 100 spans)
- Adjust flush interval if needed

## Troubleshooting

### Spans not appearing

1. **Check exporter endpoint:**
   ```python
   exporter = OTLPSpanExporter(endpoint="localhost:4317")
   print(f"Exporting to: {exporter._endpoint}")
   ```

2. **Verify OTEL receiver is running:**
   ```bash
   netstat -tuln | grep 4317
   ```

3. **Check span status:**
   ```python
   # Add logging
   logging.basicConfig(level=logging.DEBUG)
   ```

4. **Test connectivity:**
   ```bash
   grpcurl -plaintext localhost:4317 list
   ```

### Wrong layer mapping

- Explicitly set `argus.layer` attribute (don't rely on span kind)
- Verify attribute value is 1-10

### Performance degradation

- Increase batch size: `BatchSpanProcessor(exporter, max_queue_size=2048)`
- Increase flush interval: `max_export_batch_size=1000`
- Use sampling for high-volume apps

## Next Steps

- [SDK Guide](./SDK_GUIDE.md) - Argus SDK
- [Reference Apps](./REFERENCE_APPS.md) - Example implementations
- [OTEL Docs](https://opentelemetry.io/docs/) - OpenTelemetry
