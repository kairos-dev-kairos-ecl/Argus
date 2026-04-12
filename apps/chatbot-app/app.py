"""
Chatbot Application with Conversation History

Demonstrates signal emission at two layers:
- L7_RAG_RETRIEVAL: Memory/context retrieval
- L8_AGENTS: LLM inference for response generation
"""

import asyncio
import sys
import time
from typing import List, Dict, Any
from fastapi import FastAPI, HTTPException, WebSocket
from pydantic import BaseModel
import uvicorn

# Add parent directory to path for SDK import
sys.path.insert(0, '/c/Users/Drupad/ArgusXDR')

from sdk import ArgusClient, Layer, Severity, observe

# Initialize FastAPI app
app = FastAPI(title="Chatbot App", version="1.0.0")

# Initialize Argus client
argus_client = None

class ChatMessage(BaseModel):
    role: str  # "user" or "assistant"
    content: str

class ChatRequest(BaseModel):
    message: str
    session_id: str = "default"

class ChatResponse(BaseModel):
    response: str
    session_id: str

# In-memory conversation storage
CONVERSATIONS: Dict[str, List[ChatMessage]] = {}

class ConversationMemory:
    """Manage conversation history"""

    @observe(
        layer=Layer.L7_RAG_RETRIEVAL,
        category="memory.retrieval",
        client=argus_client,
    )
    def retrieve_context(self, session_id: str, max_history: int = 5) -> List[ChatMessage]:
        """Retrieve conversation history for context"""
        if session_id not in CONVERSATIONS:
            CONVERSATIONS[session_id] = []

        # Return recent messages (last N)
        return CONVERSATIONS[session_id][-max_history:]

    @observe(
        layer=Layer.L7_RAG_RETRIEVAL,
        category="memory.storage",
        client=argus_client,
    )
    def store_message(self, session_id: str, role: str, content: str) -> None:
        """Store message in conversation history"""
        if session_id not in CONVERSATIONS:
            CONVERSATIONS[session_id] = []

        CONVERSATIONS[session_id].append(ChatMessage(role=role, content=content))

class LLMResponseGenerator:
    """Generate LLM responses"""

    @observe(
        layer=Layer.L8_AGENTS,
        category="inference.chat_response",
        client=argus_client,
    )
    async def generate_response(
        self, user_message: str, context: List[ChatMessage]
    ) -> str:
        """Generate response based on user message and context"""
        await asyncio.sleep(0.05)  # Simulate inference latency

        # Build context string
        context_str = ""
        for msg in context:
            context_str += f"{msg.role}: {msg.content}\n"

        # Simulate LLM response
        return f"Thanks for asking about '{user_message}'. I'm a chatbot powered by Argus, an XDR platform for LLM systems. How can I help?"

class Chatbot:
    """Simple chatbot with conversation memory"""

    def __init__(self):
        self.memory = ConversationMemory()
        self.generator = LLMResponseGenerator()

    async def chat(self, session_id: str, user_message: str) -> str:
        """Process a chat message and generate response"""

        # Retrieve conversation context (L7)
        context = self.memory.retrieve_context(session_id)

        # Generate response (L8)
        response = await self.generator.generate_response(user_message, context)

        # Store both user message and response in memory (L7)
        self.memory.store_message(session_id, "user", user_message)
        self.memory.store_message(session_id, "assistant", response)

        return response

# Initialize chatbot
chatbot = Chatbot()

@app.on_event("startup")
async def startup_event():
    global argus_client
    argus_client = ArgusClient(
        base_url="http://localhost:8080",
        app_id="chatbot-app",
        app_version="1.0.0",
        sdk_version="0.1.0",
        environment="dev",
    )
    await argus_client.__aenter__()

@app.on_event("shutdown")
async def shutdown_event():
    if argus_client:
        await argus_client.__aexit__(None, None, None)

@app.post("/chat", response_model=ChatResponse)
async def chat(request: ChatRequest) -> ChatResponse:
    """
    Send a message and get a response
    """
    try:
        response = await chatbot.chat(request.session_id, request.message)
        return ChatResponse(response=response, session_id=request.session_id)
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/history/{session_id}")
async def get_history(session_id: str) -> Dict[str, Any]:
    """
    Get conversation history
    """
    if session_id not in CONVERSATIONS:
        return {"messages": [], "session_id": session_id}

    return {
        "messages": CONVERSATIONS[session_id],
        "session_id": session_id,
    }

@app.get("/health")
async def health_check():
    """Health check endpoint"""
    return {"status": "healthy", "service": "chatbot-app"}

@app.websocket("/ws/chat/{session_id}")
async def websocket_chat(websocket: WebSocket, session_id: str):
    """WebSocket endpoint for streaming chat"""
    await websocket.accept()

    try:
        while True:
            data = await websocket.receive_text()

            # Process message
            response = await chatbot.chat(session_id, data)

            # Send response
            await websocket.send_text(response)
    except Exception as e:
        await websocket.close(code=1000, reason=str(e))

if __name__ == "__main__":
    print("Starting Chatbot App...")
    print("Argus endpoint: http://localhost:8080")
    print("Chatbot endpoint: http://localhost:8002")
    print("WebSocket: ws://localhost:8002/ws/chat/default")
    uvicorn.run(app, host="0.0.0.0", port=8002)
