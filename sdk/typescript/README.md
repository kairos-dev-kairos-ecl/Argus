# Argus SDK for TypeScript/Node.js

Production-grade signal emission SDK for TypeScript and Node.js applications.

## Features

- **Minimal Overhead**: <5ms p99 overhead per signal
- **Fail-Open**: Continues if Argus unreachable (drop counter)
- **Middleware Pattern**: Automatic Express instrumentation
- **Async-First**: Promise-based, await-compatible
- **Trace Correlation**: Automatic trace ID propagation
- **10-Layer Coverage**: Full LLM system taxonomy support

## Installation

```bash
npm install @argusxdr/sdk
```

## Quick Start

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

// Use middleware for automatic instrumentation
app.use(argusMiddleware({ client, layer: Layer.L9_API_GATEWAY }));

// Or emit signals manually
app.post('/api/inference', async (req, res) => {
  const success = await client.emitSignal(
    Layer.L5_OUTPUT_DECODING,
    'inference.completion',
    Severity.INFO,
    {
      output_tokens: 150,
      input_tokens: 50,
      finish_reason: 'stop',
    }
  );
  res.json({ success });
});

app.listen(3000);
```

## Usage Patterns

### Express Middleware

```typescript
import { argusMiddleware, Layer } from '@argusxdr/sdk';

app.use(argusMiddleware({
  client: argusClient,
  layer: Layer.L9_API_GATEWAY,
  category: 'http.request',
  excludePaths: ['/health', '/metrics'],
  includeRequestBody: true,
  includeResponseBody: true,
}));
```

### Manual Signal Emission

```typescript
const success = await client.emitSignal(
  Layer.L8_AGENTS,
  'tool_call.execution',
  Severity.INFO,
  {
    tool_name: 'web_search',
    tool_result: '...',
    tool_latency_ms: 250,
  },
  125  // duration_ms
);

console.log(`Signal emitted: ${success}`);
```

### With Trace Correlation

```typescript
// Set trace ID from request header
client.setTraceId(req.headers['x-trace-id'] as string);

// All subsequent signals use this trace ID
await client.emitSignal(Layer.L5_OUTPUT_DECODING, ...);
```

## Configuration

```typescript
const client = new ArgusClient(
  'http://localhost:8080',  // Argus endpoint
  'my-app',                 // Application ID
  '1.0.0',                  // Version
  '0.1.0',                  // SDK version
  'production',             // Environment
  30000,                    // Timeout (ms)
);

await client.initialize();
```

## Buffering

For high-volume applications, use buffering:

```typescript
import { SignalBuffer } from '@argusxdr/sdk';

const buffer = new SignalBuffer(
  1000,   // max_size
  1.0     // flush_interval_seconds
);

await buffer.add(signal);
await buffer.startAutoFlush(emitFn);

// Monitor drops
const drops = buffer.getDropCounter();
```

## Fail-Open Behavior

The SDK gracefully handles Argus unavailability:

```typescript
// If Argus is down:
// 1. emitSignal returns false (not exception)
// 2. Drop counter increments
// 3. Application continues normally

const success = await client.emitSignal(...);
console.log(`Signal emitted: ${success}`);  // false, but no exception
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

```typescript
const start = Date.now();
await client.emitSignal(Layer.L5_OUTPUT_DECODING, 'test');
const overhead = Date.now() - start;
console.log(`Overhead: ${overhead}ms`);
```

**Target: <5ms p99 overhead**

## Testing

Run tests:

```bash
npm install
npm test
```

## API Reference

### ArgusClient

```typescript
class ArgusClient {
  constructor(
    baseUrl?: string,
    appId?: string,
    appVersion?: string,
    sdkVersion?: string,
    environment?: string,
    timeout?: number
  )

  async initialize(): Promise<void>
  async close(): Promise<void>

  async emitSignal(
    layer: Layer,
    category: string,
    severity?: Severity,
    context?: SignalContext,
    durationMs?: number,
    traceId?: string,
    parentSpanId?: string,
  ): Promise<boolean>

  setTraceId(traceId: string): void
  getTraceId(): string
}
```

### Middleware

```typescript
function argusMiddleware(options?: ArgusMiddlewareOptions): any
```

**Options:**
```typescript
interface ArgusMiddlewareOptions {
  client?: ArgusClient
  layer?: Layer
  category?: string
  severity?: Severity
  excludePaths?: string[]
  includeRequestBody?: boolean
  includeResponseBody?: boolean
}
```

## Examples

See `apps/` directory for complete examples:

- `rag-app/`: RAG pipeline (retrieval + inference)
- `agent-app/`: Agentic decision-making with tools
- `chatbot-app/`: Multi-turn conversation

All apps can be easily ported to TypeScript.

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
