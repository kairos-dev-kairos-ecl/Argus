# RAG (Retrieval-Augmented Generation) Application

A FastAPI application demonstrating signal emission across three LLM system layers using the Argus SDK.

## Architecture

The RAG pipeline demonstrates instrumentation at:

1. **L7 (RAG Retrieval)**: Vector database search for relevant documents
2. **L8 (Agents)**: LLM inference to generate answers from context
3. **L9 (API Gateway)**: Response formatting and serialization

## Signals Emitted

### L7_RAG_RETRIEVAL
- **Category**: `retrieval.vector_search`
- **Captures**: Document retrieval, search latency
- **Context**: Query text, results count, embedding model

### L8_AGENTS
- **Category**: `inference.generation`
- **Captures**: LLM inference, generation latency
- **Context**: Token counts, finish reason, temperature

### L9_API_GATEWAY
- **Category**: `response.formatting`
- **Captures**: Response serialization, formatting latency
- **Context**: Response size, format type

## Setup

### Prerequisites

- Python 3.10+
- Argus running on `http://localhost:8080`
- (Optional) Ollama with llama2 or similar model for real LLM inference

### Installation

```bash
pip install -r requirements.txt
```

### Running

```bash
python app.py
```

The app will be available at `http://localhost:8000`

## Usage

### Ask a Question

```bash
curl -X POST http://localhost:8000/ask \
  -H "Content-Type: application/json" \
  -d '{"query": "What is Argus?"}'
```

### Health Check

```bash
curl http://localhost:8000/health
```

## Observing Signals

Once the app is running and you've made requests:

1. Open the Argus dashboard at `http://localhost:3000`
2. Go to **Signal Stream**
3. Filter by `app_id = "rag-app"`
4. You should see signals from:
   - L7_RAG_RETRIEVAL (vector search)
   - L8_AGENTS (inference)
   - L9_API_GATEWAY (formatting)

## Performance Metrics

Each signal includes:

- **Duration**: Time spent in each layer (ms)
- **Trace ID**: Links signals across the RAG pipeline
- **Context**: Layer-specific metrics (token counts, search results, etc.)

## Extending

To add new retrieval methods:

1. Add `@observe()` decorator to your function
2. Specify the appropriate `Layer` and `category`
3. Include relevant context in the `context` parameter

Example:

```python
@observe(
    layer=Layer.L7_RAG_RETRIEVAL,
    category="retrieval.rerank",
    client=argus_client
)
def rerank_results(results: List[Doc]) -> List[Doc]:
    # Reranking logic
    pass
```
