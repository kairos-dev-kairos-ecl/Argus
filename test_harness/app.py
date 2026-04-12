"""
Test Harness Application — Simulates LLM + RAG + Agent workflows for Argus validation
"""

import asyncio
import time
import uuid
from typing import Any, Dict, List, Optional, Tuple
from dataclasses import dataclass
from datetime import datetime
import json

import httpx
import numpy as np
from sdk.client import ArgusClient, Layer, Severity


@dataclass
class Document:
    """RAG document chunk"""
    id: str
    text: str
    embedding: Optional[List[float]] = None
    source: str = "unknown"
    metadata: Dict[str, Any] = None

    def __post_init__(self):
        if self.metadata is None:
            self.metadata = {}


class InMemoryVectorDB:
    """Simple in-memory vector store using embeddings"""

    def __init__(self):
        self.documents: Dict[str, Document] = {}
        self.embeddings: Dict[str, np.ndarray] = {}

    def add_documents(self, docs: List[Document]):
        """Add documents to the vector store"""
        for doc in docs:
            # Mock embedding: hash-based for determinism
            embedding = self._mock_embedding(doc.text)
            doc.embedding = embedding
            self.documents[doc.id] = doc
            self.embeddings[doc.id] = embedding

    def search(self, query: str, top_k: int = 5) -> List[Tuple[Document, float]]:
        """Search for similar documents"""
        if not self.documents:
            return []

        query_embedding = self._mock_embedding(query)
        scores = []

        for doc_id, doc in self.documents.items():
            if doc.embedding:
                # Cosine similarity
                similarity = np.dot(query_embedding, doc.embedding) / (
                    np.linalg.norm(query_embedding) * np.linalg.norm(doc.embedding) + 1e-10
                )
                scores.append((doc, float(similarity)))

        scores.sort(key=lambda x: x[1], reverse=True)
        return scores[:top_k]

    def _mock_embedding(self, text: str) -> np.ndarray:
        """Generate deterministic mock embedding"""
        np.random.seed(hash(text) % 2**32)
        return np.random.randn(384).astype(np.float32)
        embedding = np.random.normal(0, 1, 384).astype(np.float32)
        embedding /= np.linalg.norm(embedding)
        return embedding


class OllamaClient:
    """Async HTTP client for Ollama local LLM"""

    def __init__(self, base_url: str = "http://localhost:11434", model: str = "llama2"):
        self.base_url = base_url.rstrip("/")
        self.model = model
        self.session: Optional[httpx.AsyncClient] = None

    async def __aenter__(self):
        self.session = httpx.AsyncClient(timeout=60.0)
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        if self.session:
            await self.session.aclose()

    async def generate(
        self,
        prompt: str,
        max_tokens: int = 256,
        temperature: float = 0.7,
        stop: Optional[List[str]] = None,
    ) -> Tuple[str, Dict[str, Any]]:
        """
        Generate text using Ollama.

        Returns:
            (text, metadata) where metadata contains token counts, timing, etc.
        """
        if not self.session:
            raise RuntimeError("Client not initialized")

        start_time = time.time()

        payload = {
            "model": self.model,
            "prompt": prompt,
            "stream": False,
            "options": {
                "temperature": temperature,
                "num_predict": max_tokens,
            },
        }

        if stop:
            payload["options"]["stop"] = stop

        try:
            response = await self.session.post(
                f"{self.base_url}/api/generate",
                json=payload,
            )
            result = response.json()

            # Extract metadata
            metadata = {
                "prompt_tokens": result.get("prompt_eval_count", 0),
                "completion_tokens": result.get("eval_count", 0),
                "total_tokens": result.get("prompt_eval_count", 0) + result.get("eval_count", 0),
                "duration_ms": (time.time() - start_time) * 1000,
                "eval_duration_ms": result.get("eval_duration", 0) / 1e6,
                "prompt_eval_duration_ms": result.get("prompt_eval_duration", 0) / 1e6,
            }

            return result.get("response", ""), metadata
        except Exception as e:
            print(f"Error calling Ollama: {e}")
            return "", {"error": str(e), "duration_ms": (time.time() - start_time) * 1000}

    async def health_check(self) -> bool:
        """Check if Ollama is running"""
        if not self.session:
            raise RuntimeError("Client not initialized")

        try:
            response = await self.session.get(f"{self.base_url}/api/tags", timeout=5.0)
            return response.status_code == 200
        except:
            return False


class ArgusTestApp:
    """Test harness that simulates LLM + RAG + Agent workflows and emits signals to Argus"""

    def __init__(
        self,
        argus_client: ArgusClient,
        ollama_client: OllamaClient,
        app_name: str = "test-harness",
    ):
        self.argus = argus_client
        self.ollama = ollama_client
        self.app_name = app_name
        self.vector_db = InMemoryVectorDB()
        self.session_id = str(uuid.uuid4())
        self.conversation_id = str(uuid.uuid4())
        self.tools = {
            "search_docs": self._tool_search_docs,
            "calculate": self._tool_calculate,
            "send_alert": self._tool_send_alert,
        }
        self.authorized_tools = {"search_docs", "calculate"}

    def setup_documents(self, docs: List[Document]):
        """Populate the RAG vector store"""
        self.vector_db.add_documents(docs)

    async def chat(self, user_input: str, system_prompt: str = "") -> str:
        """
        Simple chat interface — emits L5 (inference) and L10 (application) signals
        """
        trace_id = str(uuid.uuid4())
        self.argus.set_trace_id(trace_id)

        # L10: Application layer — user input received
        await self.argus.emit_signal(
            layer=Layer.L10_APPLICATION,
            category="chat.input",
            severity=Severity.INFO,
            context={"placeholder": f"User input: {user_input[:50]}..."},
            trace_id=trace_id,
        )

        # L5: Inference — call Ollama
        prompt = system_prompt + "\n" + user_input if system_prompt else user_input
        start = time.time()

        response_text, metadata = await self.ollama.generate(
            prompt=prompt,
            max_tokens=256,
            temperature=0.7,
        )

        duration_ms = (time.time() - start) * 1000

        # Emit L5 signal with inference metadata
        await self.argus.emit_signal(
            layer=Layer.L5_OUTPUT_DECODING,
            category="inference.completion",
            severity=Severity.INFO,
            context={
                "operation": 1,  # GENERATION
                "output_tokens": metadata.get("completion_tokens", 0),
                "input_tokens": metadata.get("prompt_tokens", 0),
                "total_tokens": metadata.get("total_tokens", 0),
                "finish_reason": "stop",
                "temperature": 0.7,
                "top_p": 1.0,
                "ttft_ms": metadata.get("prompt_eval_duration_ms", 0.0),
                "tps": metadata.get("completion_tokens", 0) / (metadata.get("eval_duration_ms", 1) / 1000 + 0.001),
            },
            duration_ms=duration_ms,
            trace_id=trace_id,
        )

        # L10: Application layer — response output
        await self.argus.emit_signal(
            layer=Layer.L10_APPLICATION,
            category="chat.output",
            severity=Severity.INFO,
            context={"placeholder": f"Response: {response_text[:50]}..."},
            trace_id=trace_id,
        )

        return response_text

    async def rag_query(
        self, query: str, system_prompt: str = "", top_k: int = 5
    ) -> Tuple[str, List[Document]]:
        """
        RAG query interface — emits L7 (retrieval) and L5 (inference) signals
        """
        trace_id = str(uuid.uuid4())
        self.argus.set_trace_id(trace_id)

        # L7: RAG Retrieval — vector search
        start = time.time()
        results = self.vector_db.search(query, top_k=top_k)
        retrieval_time_ms = (time.time() - start) * 1000

        retrieved_docs = [doc for doc, _ in results]
        retrieved_text = "\n".join([doc.text for doc in retrieved_docs])

        # Emit L7 signal
        await self.argus.emit_signal(
            layer=Layer.L7_RAG_RETRIEVAL,
            category="retrieval.search",
            severity=Severity.INFO,
            context={
                "operation": 1,  # VECTOR_SEARCH
                "query_text": query,
                "results_count": len(retrieved_docs),
                "embedding_model": "mock-embedder",
                "vector_index": "in-memory",
                "context_window_pct": len(retrieved_text) / (4096 * 0.5),  # Rough estimate
            },
            duration_ms=retrieval_time_ms,
            trace_id=trace_id,
        )

        # L5: Inference — augmented generation
        augmented_prompt = f"Context:\n{retrieved_text}\n\nQuery: {query}"
        if system_prompt:
            augmented_prompt = system_prompt + "\n" + augmented_prompt

        start = time.time()
        response_text, metadata = await self.ollama.generate(
            prompt=augmented_prompt,
            max_tokens=256,
        )
        inference_time_ms = (time.time() - start) * 1000

        # Emit L5 signal
        await self.argus.emit_signal(
            layer=Layer.L5_OUTPUT_DECODING,
            category="inference.completion",
            severity=Severity.INFO,
            context={
                "operation": 1,
                "output_tokens": metadata.get("completion_tokens", 0),
                "input_tokens": metadata.get("prompt_tokens", 0),
                "total_tokens": metadata.get("total_tokens", 0),
                "finish_reason": "stop",
                "temperature": 0.7,
                "top_p": 1.0,
            },
            duration_ms=inference_time_ms,
            trace_id=trace_id,
        )

        return response_text, retrieved_docs

    async def agent_task(
        self, goal: str, max_iterations: int = 5
    ) -> Tuple[str, int]:
        """
        Simple agent loop — emits L8 (agent) signals for tool calls
        Returns (final_result, iterations_used)
        """
        trace_id = str(uuid.uuid4())
        self.argus.set_trace_id(trace_id)

        context = {"goal": goal, "messages": []}
        iteration = 0

        for iteration in range(max_iterations):
            # Decide on next action (mock decision)
            action = self._decide_next_action(goal, iteration, len(context["messages"]))

            if action == "done":
                break

            # Execute tool
            tool_name, tool_args = action
            result = await self._execute_tool(
                trace_id=trace_id,
                tool_name=tool_name,
                tool_args=tool_args,
                step_number=iteration + 1,
                max_steps=max_iterations,
            )

            context["messages"].append({
                "iteration": iteration,
                "tool": tool_name,
                "args": tool_args,
                "result": result,
            })

        final_result = self._summarize_result(context)
        return final_result, iteration + 1

    async def _execute_tool(
        self,
        trace_id: str,
        tool_name: str,
        tool_args: Dict[str, Any],
        step_number: int,
        max_steps: int,
    ) -> str:
        """Execute a tool and emit L8 agent signal"""
        start = time.time()

        # Check authorization
        is_authorized = tool_name in self.authorized_tools
        if not is_authorized:
            result = f"ERROR: Tool '{tool_name}' is not authorized"
        else:
            tool_fn = self.tools.get(tool_name)
            result = tool_fn(**tool_args) if tool_fn else f"Unknown tool: {tool_name}"

        duration_ms = (time.time() - start) * 1000

        # Emit L8 agent signal
        permissions_requested = []
        if not is_authorized:
            permissions_requested = [f"execute:{tool_name}"]

        await self.argus.emit_signal(
            layer=Layer.L8_AGENTS,
            category="agent.tool_call",
            severity=Severity.HIGH if not is_authorized else Severity.INFO,
            context={
                "operation": 1,  # TOOL_CALL
                "tool_name": tool_name,
                "tool_provider": "internal",
                "tool_arguments": tool_args,
                "tool_result": result[:100] if is_authorized else "",
                "tool_error": "" if is_authorized else "Unauthorized",
                "tool_latency_ms": duration_ms,
                "step_number": step_number,
                "total_steps": max_steps,
                "permissions_used": [f"execute:{tool_name}"] if is_authorized else [],
                "permissions_requested": permissions_requested,
                "data_flow_tags": ["pii_possible"] if "send_alert" in tool_name else [],
            },
            trace_id=trace_id,
        )

        return result

    def _tool_search_docs(self, query: str) -> str:
        """Mock tool: search documents"""
        results = self.vector_db.search(query, top_k=3)
        if not results:
            return "No documents found"
        return "; ".join([f"{doc.source}: {doc.text[:30]}" for doc, _ in results])

    def _tool_calculate(self, expression: str) -> str:
        """Mock tool: evaluate mathematical expression"""
        try:
            result = eval(expression, {"__builtins__": {}}, {})
            return str(result)
        except:
            return f"Invalid expression: {expression}"

    def _tool_send_alert(self, severity: str, message: str) -> str:
        """Mock tool: send alert"""
        return f"Alert ({severity}): {message}"

    def _decide_next_action(self, goal: str, iteration: int, messages_count: int) -> Any:
        """Decide next tool or action"""
        if iteration >= 2 or messages_count >= 2:
            return "done"
        if iteration == 0:
            return ("search_docs", {"query": goal})
        else:
            return ("calculate", {"expression": "2 + 2"})

    def _summarize_result(self, context: Dict[str, Any]) -> str:
        """Summarize agent execution"""
        return f"Completed {len(context['messages'])} steps for goal: {context['goal']}"


async def main():
    """Test the app"""
    async with ArgusClient("http://localhost:8080") as argus:
        async with OllamaClient("http://localhost:11434") as ollama:
            app = ArgusTestApp(argus, ollama)

            # Test health check
            if not await ollama.health_check():
                print("ERROR: Ollama not running. Start with: ollama serve")
                return

            # Setup RAG documents
            docs = [
                Document(
                    id="doc1",
                    text="Python is a high-level programming language",
                    source="wiki",
                ),
                Document(
                    id="doc2",
                    text="Machine learning is a subset of artificial intelligence",
                    source="textbook",
                ),
            ]
            app.setup_documents(docs)

            # Test chat
            print("Testing chat...")
            response = await app.chat("What is Python?")
            print(f"Response: {response[:100]}...")

            # Test RAG
            print("Testing RAG...")
            response, docs = await app.rag_query("What is machine learning?")
            print(f"Response: {response[:100]}...")
            print(f"Retrieved {len(docs)} documents")

            # Test agent
            print("Testing agent...")
            result, steps = await app.agent_task("Calculate 5 * 5")
            print(f"Result: {result}, Steps: {steps}")


if __name__ == "__main__":
    asyncio.run(main())
