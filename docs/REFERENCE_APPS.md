# Reference Applications Guide

Three production-ready reference applications demonstrating signal emission across the LLM system layers.

## Quick Start

All three apps require Argus running on `http://localhost:8080`:

```bash
# Terminal 1: Start Argus
cd /path/to/argus
docker-compose up

# Terminal 2: Start Argus dashboard (if not auto-started)
cd /path/to/web
npm start

# Terminal 3: Start RAG app
cd apps/rag-app
pip install -r requirements.txt
python app.py

# Terminal 4: Start Agent app
cd apps/agent-app
pip install -r requirements.txt
python app.py

# Terminal 5: Start Chatbot app
cd apps/chatbot-app
pip install -r requirements.txt
python app.py
```

Then open the Argus dashboard: http://localhost:3000

## 1. RAG Application

**Purpose:** Demonstrates document retrieval + LLM inference pipeline

**URL:** http://localhost:8000

**Signals Emitted:**
- L7 (RAG Retrieval): `retrieval.vector_search`
- L8 (Agents): `inference.generation`
- L9 (API Gateway): `response.formatting`

### Usage

```bash
# Ask a question
curl -X POST http://localhost:8000/ask \
  -H "Content-Type: application/json" \
  -d '{"query": "What is Argus?"}'

# Response:
# {
#   "answer": "Based on the context: ...",
#   "sources": ["doc-1", "doc-2"],
#   "confidence": 0.95
# }
```

### What It Does

1. **Retrieval (L7):** Searches in-memory vector DB for relevant documents
   - Emits `retrieval.vector_search` signal
   - Captures: query text, results count, embedding model

2. **Inference (L8):** Generates answer using context
   - Emits `inference.generation` signal
   - Captures: generation latency, token counts

3. **Formatting (L9):** Serializes response
   - Emits `response.formatting` signal
   - Captures: formatting latency

### Observing Signals

In Argus dashboard:

```sql
SELECT timestamp, duration_ms, category
FROM signals
WHERE app_id = 'rag-app'
ORDER BY timestamp DESC
LIMIT 20
```

Expected output:
```
2024-01-15 12:34:56  | 12.5ms | retrieval.vector_search
2024-01-15 12:34:57  | 45.2ms | inference.generation
2024-01-15 12:34:57  | 2.1ms  | response.formatting
```

### Extending

Add new document types:

```python
# In app.py
DOCUMENTS = [
    # ... existing documents ...
    {
        "id": "doc-4",
        "text": "Your custom document",
        "embedding": [0.1, 0.2, ...],
    },
]
```

## 2. Agent Application

**Purpose:** Demonstrates agentic behavior with tool calling

**URL:** http://localhost:8001

**Signals Emitted:**
- L7 (RAG): `retrieval.knowledge_lookup`
- L8 (Agents): `reasoning.planning`
- L8 (Agents): `decision.tool_selection`
- L8 (Agents): `tool_call.execution`

### Usage

```bash
# Run agent on a task
curl -X POST http://localhost:8001/run-agent \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Tell me about Argus",
    "max_steps": 5
  }'

# Response:
# {
#   "result": "Based on knowledge base lookup...",
#   "steps_taken": 2,
#   "trace_id": "abc123..."
# }

# List available tools
curl http://localhost:8001/tools
```

### What It Does

1. **Planning (L8):** Creates a plan for the task
   - Emits `reasoning.planning` signal
   - Captures: task breakdown, tool selection strategy

2. **Tool Selection (L8):** Selects the best tool
   - Emits `decision.tool_selection` signal
   - Captures: selected tool, arguments, confidence

3. **Tool Execution (L8):** Executes the selected tool
   - Emits `tool_call.execution` signal
   - Captures: tool name, arguments, latency

4. **Knowledge Lookup (L7):** Retrieves from knowledge base
   - Emits `retrieval.knowledge_lookup` signal
   - Captures: query, results

### Available Tools

- `search` - Search the knowledge base
- `calculate` - Perform math (e.g., "10 + 5")
- `lookup` - Look up specific terms

### Observing Signal Trace

All signals from one agent run share the same `trace_id`:

```sql
SELECT category, duration_ms, context
FROM signals
WHERE app_id = 'agent-app' AND trace_id = 'abc123...'
ORDER BY timestamp
```

Expected flow:
```
reasoning.planning (5ms)
  ↓
decision.tool_selection (8ms)
  ↓
tool_call.execution (45ms)
  ↓
[result returned]
```

### Extending

Add new tools:

```python
# In app.py, ToolExecutor class
def execute_tool(self, tool_name: str, args: Dict[str, Any]) -> str:
    if tool_name == "my_tool":
        return "Tool result"
```

## 3. Chatbot Application

**Purpose:** Demonstrates multi-turn conversation with memory

**URL:** http://localhost:8002

**Signals Emitted:**
- L7 (RAG): `memory.retrieval`
- L7 (RAG): `memory.storage`
- L8 (Agents): `inference.chat_response`

### Usage

```bash
# Single message (stateless)
curl -X POST http://localhost:8002/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Hello, how are you?",
    "session_id": "user-123"
  }'

# Get conversation history
curl http://localhost:8002/history/user-123

# Response:
# {
#   "messages": [
#     {"role": "user", "content": "Hello..."},
#     {"role": "assistant", "content": "Thanks for asking..."}
#   ]
# }

# WebSocket (streaming)
websocat ws://localhost:8002/ws/chat/user-123
# Then type messages to get responses
```

### What It Does

1. **Memory Retrieval (L7):** Retrieves conversation history
   - Emits `memory.retrieval` signal
   - Captures: session ID, history size, context window

2. **Response Generation (L8):** Generates response
   - Emits `inference.chat_response` signal
   - Captures: input/output tokens, latency

3. **Memory Storage (L7):** Stores new messages
   - Emits `memory.storage` signal
   - Captures: session ID, message size

### Session Management

Same `session_id` maintains conversation:

```bash
# Message 1
curl -X POST http://localhost:8002/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "What is Argus?",
    "session_id": "chat-1"
  }'

# Message 2 (same session = has context from message 1)
curl -X POST http://localhost:8002/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Tell me more",
    "session_id": "chat-1"
  }'

# Message 3 (different session = no context)
curl -X POST http://localhost:8002/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "What is Argus?",
    "session_id": "chat-2"
  }'
```

### Observing Signals

Check memory operations per session:

```sql
SELECT category, duration_ms
FROM signals
WHERE app_id = 'chatbot-app'
  AND category IN ('memory.retrieval', 'memory.storage', 'inference.chat_response')
ORDER BY timestamp
LIMIT 50
```

## Cross-App Correlation

Since all apps emit signals to the same Argus instance, you can correlate across them:

```sql
-- Find all signals from all reference apps
SELECT app_id, category, duration_ms
FROM signals
WHERE app_id IN ('rag-app', 'agent-app', 'chatbot-app')
ORDER BY timestamp DESC
LIMIT 100

-- Find slowest operations
SELECT app_id, category, MAX(duration_ms) as max_latency
FROM signals
WHERE timestamp > now() - interval '1 hour'
GROUP BY app_id, category
ORDER BY max_latency DESC

-- Compare layer distribution
SELECT app_id, layer, COUNT(*) as signal_count
FROM signals
WHERE timestamp > now() - interval '1 hour'
GROUP BY app_id, layer
```

## Performance Metrics

Run concurrent load tests:

```bash
# Load test RAG app (10 requests per second for 60 seconds)
ab -n 600 -c 10 -p request.json -T application/json \
  http://localhost:8000/ask

# Load test Agent app
ab -n 600 -c 10 -p agent-request.json -T application/json \
  http://localhost:8001/run-agent

# Load test Chatbot app
ab -n 600 -c 10 -p chat-request.json -T application/json \
  http://localhost:8002/chat
```

Check metrics in Argus dashboard:
- Average latency per app
- P95/P99 latencies
- Error rates
- Signal emission rate (signals/sec)

## Troubleshooting

### Signals not appearing

1. **Is Argus running?**
   ```bash
   curl http://localhost:8080/health
   ```

2. **Is dashboard refreshing?**
   - Open browser console for WebSocket errors
   - Check that Signal Stream shows activity

3. **Check app logs:**
   ```bash
   # Should show "Signal emitted: True"
   tail -f /tmp/app.log
   ```

### Connection refused

- Verify apps are on correct ports (8000, 8001, 8002)
- Check firewall/network
- Restart apps if needed

### High latency

1. Check if Argus ingestion is backed up
2. Check network latency between app and Argus
3. Check CPU/memory on Argus

## Next Steps

- [SDK Guide](./SDK_GUIDE.md) - SDK documentation
- [OpenTelemetry Integration](./OTEL_INTEGRATION.md) - OTEL bridge
- [Performance Baseline](./PERFORMANCE_BASELINE.md) - Benchmark data
