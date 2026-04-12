# Chatbot Application

A FastAPI application demonstrating multi-turn conversation with signal emission across memory and inference layers.

## Architecture

The chatbot demonstrates instrumentation at:

1. **L7 (RAG Retrieval)**: Conversation memory retrieval and storage
2. **L8 (Agents)**: LLM inference for response generation

## Signals Emitted

### L7_RAG_RETRIEVAL - Memory Management
- **Category**: `memory.retrieval`
- **Captures**: Conversation history retrieval
- **Context**: Session ID, history size, time window

### L7_RAG_RETRIEVAL - Memory Storage
- **Category**: `memory.storage`
- **Captures**: Storing new messages in history
- **Context**: Session ID, message size, total messages

### L8_AGENTS - Response Generation
- **Category**: `inference.chat_response`
- **Captures**: LLM inference for chat response
- **Context**: Input tokens, output tokens, latency

## Features

- Multi-turn conversations with persistent history
- Per-session conversation tracking
- REST API for stateless requests
- WebSocket endpoint for real-time streaming
- Automatic signal emission at L7 and L8

## Setup

### Prerequisites

- Python 3.10+
- Argus running on `http://localhost:8080`

### Installation

```bash
pip install -r requirements.txt
```

### Running

```bash
python app.py
```

The app will be available at:
- REST: `http://localhost:8002`
- WebSocket: `ws://localhost:8002/ws/chat/default`

## Usage

### REST API - Single Message

```bash
curl -X POST http://localhost:8002/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Hello, how are you?",
    "session_id": "user-123"
  }'
```

### Get Conversation History

```bash
curl http://localhost:8002/history/user-123
```

### WebSocket - Real-time Chat

```javascript
const ws = new WebSocket('ws://localhost:8002/ws/chat/user-123');

ws.onopen = () => {
  ws.send('Hello, chatbot!');
};

ws.onmessage = (event) => {
  console.log('Response:', event.data);
};
```

### Health Check

```bash
curl http://localhost:8002/health
```

## Observing Signals

Once the app is running and you've made requests:

1. Open the Argus dashboard at `http://localhost:3000`
2. Go to **Signal Stream**
3. Filter by `app_id = "chatbot-app"`
4. You should see signals from:
   - L7_RAG_RETRIEVAL (memory retrieval and storage)
   - L8_AGENTS (response generation)

## Conversation Management

### Session IDs

Each conversation is identified by a `session_id`. Use the same session ID to maintain conversation history:

```bash
# First message
curl -X POST http://localhost:8002/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "What is Argus?",
    "session_id": "chat-1"
  }'

# Second message (same session)
curl -X POST http://localhost:8002/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Tell me more",
    "session_id": "chat-1"
  }'
```

## Performance Metrics

Each signal includes:

- **Duration**: Time spent in memory or inference (ms)
- **Trace ID**: Links all signals in a single conversation
- **Context**: Session details, message counts, token usage

## Extending

To add custom response logic:

1. Modify `LLMResponseGenerator.generate_response()`
2. Update context retrieval in `ConversationMemory`
3. Add new signal categories as needed

Example:

```python
@observe(
    layer=Layer.L8_AGENTS,
    category="inference.custom_model",
    client=argus_client
)
async def generate_custom_response(self, message: str) -> str:
    # Custom inference logic
    pass
```
