# Argus XDR — Backend Validation Harness

One-command end-to-end validation that runs the full Argus backend plus a local
llama.cpp instance, drives LLM requests through an instrumented SDK client, and
confirms via ClickHouse queries which of the 10 signal layers make it through
the ingest → pipeline → storage path.

## Prerequisites

- Docker Compose v2
- Python 3.11+
- `huggingface-cli` (install: `pip install huggingface_hub[cli]`)

## Step 1: Download the model

```bash
huggingface-cli download Qwen/Qwen2.5-0.5B-Instruct-GGUF \
  qwen2.5-0.5b-instruct-q4_k_m.gguf \
  --local-dir test_harness/models/
```

This downloads ~300 MB into `test_harness/models/`. The `.gguf` files are
gitignored and will never be committed.

## Step 2: Run the harness

```bash
bash test_harness/run_harness.sh
```

This will:
1. Verify the model file exists
2. Start all 5 Docker services (ClickHouse, PostgreSQL, Redis, Argus, llama.cpp)
3. Wait up to 120s for all services to become healthy
4. Drive 5 test prompts through the instrumented LLM client
5. Wait for batch writer to flush (10s)
6. Run the ClickHouse validator and produce `test_harness/validation_results.md`

## Step 3: Teardown

```bash
bash test_harness/teardown.sh
```

Removes all test containers and volumes. The dev stack (`docker-compose.yml`) is
untouched throughout.

## Services and ports

| Service              | Container              | Host port |
|----------------------|------------------------|-----------|
| ClickHouse (native)  | argus-clickhouse-test  | 9001      |
| ClickHouse (HTTP)    | argus-clickhouse-test  | 8124      |
| PostgreSQL           | argus-postgres-test    | 5433      |
| Redis                | argus-redis-test       | 6380      |
| Argus API            | argus-server-test      | 8082      |
| llama.cpp            | argus-llamacpp-test    | 8081      |

## Signal coverage

The harness emits one ArgusSignal per layer per prompt (11 layers × 5 prompts =
55 minimum signals). Layers emitted:

| Layer      | Description           | Fields populated                        |
|------------|-----------------------|-----------------------------------------|
| L1         | Hardware              | CPU %, RAM (via psutil)                 |
| L2         | Model weights         | model_id, quantization, model_size_gb   |
| L3         | Tokenizer             | input_tokens, total_tokens, context_win |
| L4         | Transformer           | sequence_length, batch_size             |
| L5         | Output decoding       | tokens_out, latency_ms, finish_reason   |
| L6         | Safety                | safety_score, safe                      |
| L7         | RAG retrieval         | skipped — no retrieval in this harness  |
| L8         | Agents                | session_id, turn_number                 |
| L9         | Network / API gateway | client_ip, path, status_code            |
| L10        | Application           | app_id, user_id, session_id             |
| LDecision  | Decision log          | reasoning, confidence                   |
