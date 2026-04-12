# OpenTelemetry Integration

Convert OpenTelemetry spans to Argus signals via OTLP receiver.

## Overview

The OTEL integration allows you to:

1. Export traces from your application using the OpenTelemetry SDK
2. Send them to the Argus OTLP receiver (gRPC port 4317)
3. Automatically convert OTLP spans to Argus signals
4. Correlate signals across your LLM system

## Components

### OTLPToArgusConverter
Converts OTLP spans to ArgusSignal protobuf messages.

**Span Kind → Layer Mapping:**
- INTERNAL (1) → L8_AGENTS
- SERVER (2) → L7_RAG_RETRIEVAL
- CLIENT (3) → L9_API_GATEWAY
- PRODUCER (4) → L5_OUTPUT_DECODING
- CONSUMER (5) → L4_TRANSFORMER

### OTLPReceiver
gRPC server that receives OTLP trace exports and converts them to Argus signals.

**Features:**
- Batch processing (configurable batch size)
- Automatic periodic flushing
- Fail-open behavior when Argus unreachable
- Support for span attributes mapping

### OTLPGRPCReceiver
Simplified gRPC service implementation for OTLP TraceService.

## Setup

### Installation

```bash
pip install opentelemetry-api opentelemetry-sdk opentelemetry-exporter-otlp-proto-grpc
```

### Starting the Receiver

```python
from sdk import ArgusClient
from sdk.otel import OTLPReceiver

# Initialize Argus client
argus_client = ArgusClient()
await argus_client.initialize()

# Start OTLP receiver
receiver = OTLPReceiver(argus_client, app_id="my-app")
await receiver.start_periodic_flush()

# Server will listen on 0.0.0.0:4317 for OTLP gRPC requests
```

## Usage

### Example: FastAPI with OTEL

```python
from fastapi import FastAPI
from opentelemetry import trace, metrics
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.sdk.trace.export import BatchSpanProcessor

app = FastAPI()

# Configure OTEL tracer
exporter = OTLPSpanExporter(endpoint="localhost:4317")
trace.set_tracer_provider(TracerProvider())
trace.get_tracer_provider().add_span_processor(BatchSpanProcessor(exporter))

tracer = trace.get_tracer(__name__)

@app.post("/api/endpoint")
async def my_endpoint():
    with tracer.start_as_current_span("process_request"):
        # Your logic here
        pass
    return {"status": "ok"}
```

### Span Attributes for Layer Mapping

Set these attributes on your spans to map to specific Argus layers:

```python
with tracer.start_as_current_span("search") as span:
    # Set layer explicitly
    span.set_attribute("argus.layer", 7)  # L7_RAG_RETRIEVAL
    span.set_attribute("argus.category", "retrieval.search")
    
    # Add layer-specific context
    span.set_attribute("retrieval.operation", 1)
    span.set_attribute("retrieval.query", "search term")
    span.set_attribute("retrieval.results_count", 10)
    span.set_attribute("retrieval.embedding_model", "sentence-transformers/all-MiniLM-L6-v2")
    
    # Service metadata
    span.set_attribute("service.name", "my-rag-service")
    span.set_attribute("service.version", "1.0.0")
    span.set_attribute("deployment.environment", "production")
```

### Span Attribute Reference

**Common Attributes:**
- `argus.layer` (int): Explicit layer override (1-10)
- `argus.category` (string): Signal category

**L5 (Output Decoding) Attributes:**
- `llm.operation` (int): Operation type
- `llm.input_tokens` (int): Input token count
- `llm.output_tokens` (int): Output token count
- `llm.finish_reason` (string): Finish reason
- `llm.temperature` (float): Temperature
- `llm.top_p` (float): Top-p value

**L7 (RAG Retrieval) Attributes:**
- `retrieval.operation` (int): Operation type
- `retrieval.query` (string): Query text
- `retrieval.results_count` (int): Number of results
- `retrieval.embedding_model` (string): Embedding model
- `retrieval.vector_index` (string): Vector index name

**L8 (Agents) Attributes:**
- `agent.operation` (int): Operation type
- `agent.tool_name` (string): Tool name
- `agent.tool_provider` (string): Tool provider
- `agent.tool_latency_ms` (float): Tool latency
- `agent.step_number` (int): Agent step number
- `agent.total_steps` (int): Total steps planned

**Service Attributes:**
- `service.name` (string): Application name
- `service.version` (string): Application version
- `service.instance.id` (string): Instance ID
- `deployment.environment` (string): Environment (dev/staging/prod)

## Signal Conversion

### Example Span → Signal

**Input OTLP Span:**
```json
{
  "traceId": "abc123...",
  "spanId": "xyz789...",
  "name": "vector_search",
  "kind": 2,
  "startTimeUnixNano": 1704067200000000000,
  "endTimeUnixNano": 1704067200050000000,
  "attributes": [
    {"key": "argus.layer", "value": {"intValue": 7}},
    {"key": "argus.category", "value": {"stringValue": "retrieval.search"}},
    {"key": "retrieval.results_count", "value": {"intValue": 10}}
  ]
}
```

**Output ArgusSignal:**
```protobuf
signal_id: "1704067200000abc123xyz"
trace_id: "abc123..."
span_id: "xyz789..."
layer: 7  # L7_RAG_RETRIEVAL
category: "retrieval.search"
severity: 1  # INFO
timestamp: 1704067200.000
duration_ms: 50
context_l7 {
  operation: 1  # VECTOR_SEARCH
  results_count: 10
}
```

## Performance

- **Batch processing**: Configurable batch size (default 100 spans)
- **Periodic flushing**: Configurable interval (default 1 second)
- **Fail-open**: Continues accepting spans if Argus unreachable
- **Low overhead**: Minimal processing before converting to Argus format

## Troubleshooting

### Spans not appearing in Argus

1. Check that OTLP exporter is pointing to `localhost:4317`
2. Verify receiver is started and running
3. Check that `argus.layer` or `argus.category` are set (or span kind will be used)
4. Ensure Argus signal endpoint is reachable

### Missing attributes in signals

- Verify span attributes are set before span completion
- Check attribute key names match expected format
- Some attributes may be dropped if invalid or truncated

## Integration with OpenTelemetry Collector

For production deployments, integrate with the official OpenTelemetry Collector:

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

processors:
  batch:
    send_batch_size: 100
    timeout: 10s

exporters:
  otlp:
    endpoint: localhost:9200  # Argus OTLP receiver
    headers:
      authorization: Bearer <token>

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlp]
```

## References

- [OpenTelemetry Protocol](https://opentelemetry.io/docs/specs/otel/protocol/)
- [Trace Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/trace/)
- [OpenTelemetry Python](https://opentelemetry.io/docs/instrumentation/python/)
