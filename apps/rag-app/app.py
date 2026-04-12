"""
RAG (Retrieval-Augmented Generation) Application

Demonstrates signal emission at three layers:
- L7_RAG_RETRIEVAL: Vector DB search
- L8_AGENTS: LLM inference
- L9_API_GATEWAY: Response formatting
"""

import asyncio
import sys
import time
from typing import List, Dict, Any
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
import uvicorn

# Add parent directory to path for SDK import
sys.path.insert(0, '/c/Users/Drupad/ArgusXDR')

from sdk import ArgusClient, Layer, Severity, observe

# Initialize FastAPI app
app = FastAPI(title="RAG App", version="1.0.0")

# Initialize Argus client
argus_client = None

class RAGRequest(BaseModel):
    query: str

class RAGResponse(BaseModel):
    answer: str
    sources: List[str]
    confidence: float

# In-memory vector DB (simple for testing)
DOCUMENTS = [
    {
        "id": "doc-1",
        "text": "Argus is an XDR platform for LLM systems",
        "embedding": [0.1, 0.2, 0.3, 0.4, 0.5],
    },
    {
        "id": "doc-2",
        "text": "Extended Detection and Response (XDR) provides comprehensive security",
        "embedding": [0.2, 0.3, 0.4, 0.5, 0.6],
    },
    {
        "id": "doc-3",
        "text": "Signal detection uses machine learning models",
        "embedding": [0.3, 0.4, 0.5, 0.6, 0.7],
    },
]

class VectorDB:
    """Simple in-memory vector database"""

    @observe(
        layer=Layer.L7_RAG_RETRIEVAL,
        category="retrieval.vector_search",
        severity=Severity.INFO,
        client=argus_client,
    )
    def search(self, query: str, top_k: int = 3) -> List[Dict[str, Any]]:
        """Search for documents matching the query"""
        # Simulate vector search by returning top documents
        results = DOCUMENTS[:top_k]
        return results

class LLMInferencer:
    """Simulate LLM inference"""

    @observe(
        layer=Layer.L8_AGENTS,
        category="inference.generation",
        severity=Severity.INFO,
        client=argus_client,
    )
    async def generate_answer(
        self, query: str, context: List[Dict[str, Any]]
    ) -> str:
        """Generate answer using LLM"""
        await asyncio.sleep(0.1)  # Simulate inference latency

        # Simulate LLM response
        context_str = "\n".join([doc["text"] for doc in context])
        return f"Based on the context: {context_str}. The answer to '{query}' is that Argus provides comprehensive signal detection for LLM systems."

class ResponseFormatter:
    """Format the final response"""

    @observe(
        layer=Layer.L9_API_GATEWAY,
        category="response.formatting",
        severity=Severity.INFO,
        client=argus_client,
    )
    def format_response(
        self, answer: str, sources: List[str], confidence: float
    ) -> RAGResponse:
        """Format response for API"""
        return RAGResponse(answer=answer, sources=sources, confidence=confidence)

# Initialize components
vector_db = VectorDB()
llm = LLMInferencer()
formatter = ResponseFormatter()

@app.on_event("startup")
async def startup_event():
    global argus_client
    argus_client = ArgusClient(
        base_url="http://localhost:8080",
        app_id="rag-app",
        app_version="1.0.0",
        sdk_version="0.1.0",
        environment="dev",
    )
    await argus_client.__aenter__()

@app.on_event("shutdown")
async def shutdown_event():
    if argus_client:
        await argus_client.__aexit__(None, None, None)

@app.post("/ask", response_model=RAGResponse)
async def ask(request: RAGRequest) -> RAGResponse:
    """
    Ask a question and get a RAG-generated answer
    """
    try:
        # Step 1: Retrieve relevant documents (L7)
        start = time.time()
        documents = vector_db.search(request.query, top_k=3)
        retrieval_time = time.time() - start

        # Step 2: Generate answer using LLM (L8)
        start = time.time()
        answer = await llm.generate_answer(request.query, documents)
        generation_time = time.time() - start

        # Step 3: Format response (L9)
        sources = [doc["id"] for doc in documents]
        confidence = 0.95

        start = time.time()
        response = formatter.format_response(answer, sources, confidence)
        format_time = time.time() - start

        return response

    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/health")
async def health_check():
    """Health check endpoint"""
    return {"status": "healthy", "service": "rag-app"}

if __name__ == "__main__":
    print("Starting RAG App...")
    print("Argus endpoint: http://localhost:8080")
    print("RAG endpoint: http://localhost:8000")
    uvicorn.run(app, host="0.0.0.0", port=8000)
