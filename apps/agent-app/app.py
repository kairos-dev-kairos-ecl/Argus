"""
Agent Application with Tool Calling

Demonstrates signal emission at three layers:
- L8_AGENTS: Agent reasoning and decision-making
- L8_AGENTS: Tool invocation
- L9_API_GATEWAY: Final decision and response
"""

import asyncio
import json
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
app = FastAPI(title="Agent App", version="1.0.0")

# Initialize Argus client
argus_client = None

class AgentRequest(BaseModel):
    task: str
    max_steps: int = 5

class AgentResponse(BaseModel):
    result: str
    steps_taken: int
    trace_id: str

# Available tools for the agent
TOOLS = {
    "search": "Search the knowledge base",
    "calculate": "Perform mathematical calculations",
    "lookup": "Look up information",
}

class KnowledgeBase:
    """Simple knowledge base"""

    @observe(
        layer=Layer.L7_RAG_RETRIEVAL,
        category="retrieval.knowledge_lookup",
        client=argus_client,
    )
    def lookup(self, query: str) -> str:
        """Look up information in knowledge base"""
        kb = {
            "argus": "Argus is an XDR platform for LLM systems",
            "xdr": "XDR (Extended Detection and Response) provides comprehensive security",
            "signal": "Signals are events from any layer of an LLM system",
            "detection": "Detection rules identify anomalies and threats",
        }
        return kb.get(query.lower(), "Information not found")

class ToolExecutor:
    """Execute agent tools"""

    @observe(
        layer=Layer.L8_AGENTS,
        category="tool_call.execution",
        client=argus_client,
    )
    def execute_tool(self, tool_name: str, args: Dict[str, Any]) -> str:
        """Execute a tool"""
        if tool_name == "search":
            return f"Search results for: {args.get('query', '')}"
        elif tool_name == "calculate":
            try:
                result = eval(args.get('expression', '0'))
                return str(result)
            except:
                return "Calculation error"
        elif tool_name == "lookup":
            kb = KnowledgeBase()
            return kb.lookup(args.get('query', ''))
        else:
            return "Unknown tool"

class Agent:
    """Simple agent that uses tools to solve tasks"""

    def __init__(self):
        self.tool_executor = ToolExecutor()
        self.kb = KnowledgeBase()

    @observe(
        layer=Layer.L8_AGENTS,
        category="reasoning.planning",
        client=argus_client,
    )
    def plan(self, task: str) -> List[str]:
        """Create a plan for the task"""
        # Simulate planning by selecting tools
        if "calculate" in task.lower():
            return ["calculate"]
        elif "search" in task.lower():
            return ["search"]
        else:
            return ["lookup"]

    @observe(
        layer=Layer.L8_AGENTS,
        category="decision.tool_selection",
        client=argus_client,
    )
    def select_tool(self, task: str) -> tuple[str, Dict[str, Any]]:
        """Select the best tool for the task"""
        tools_list = list(TOOLS.keys())
        selected = tools_list[0] if tools_list else None

        args = {}
        if "calculate" in task.lower():
            args = {"expression": "10 + 5"}
        else:
            args = {"query": task}

        return (selected or "lookup", args)

    async def run(self, task: str, max_steps: int = 5) -> tuple[str, int]:
        """Run the agent"""
        steps = 0

        for step in range(max_steps):
            steps += 1

            # Step 1: Plan (L8)
            plan = self.plan(task)

            # Step 2: Select tool (L8)
            tool_name, tool_args = self.select_tool(task)

            # Step 3: Execute tool
            result = self.tool_executor.execute_tool(tool_name, tool_args)

            # Check if we have a good result
            if result and result != "Information not found":
                return (result, steps)

            await asyncio.sleep(0.01)

        return ("Unable to solve task", steps)

# Initialize agent
agent = Agent()

@app.on_event("startup")
async def startup_event():
    global argus_client
    argus_client = ArgusClient(
        base_url="http://localhost:8080",
        app_id="agent-app",
        app_version="1.0.0",
        sdk_version="0.1.0",
        environment="dev",
    )
    await argus_client.__aenter__()

@app.on_event("shutdown")
async def shutdown_event():
    if argus_client:
        await argus_client.__aexit__(None, None, None)

@app.post("/run-agent", response_model=AgentResponse)
async def run_agent(request: AgentRequest) -> AgentResponse:
    """
    Run the agent on a task
    """
    try:
        trace_id = argus_client.get_trace_id() if argus_client else "unknown"

        # Run the agent (L8)
        result, steps = await agent.run(request.task, request.max_steps)

        return AgentResponse(result=result, steps_taken=steps, trace_id=trace_id)

    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/health")
async def health_check():
    """Health check endpoint"""
    return {"status": "healthy", "service": "agent-app"}

@app.get("/tools")
async def list_tools():
    """List available tools"""
    return {"tools": TOOLS}

if __name__ == "__main__":
    print("Starting Agent App...")
    print("Argus endpoint: http://localhost:8080")
    print("Agent endpoint: http://localhost:8001")
    uvicorn.run(app, host="0.0.0.0", port=8001)
