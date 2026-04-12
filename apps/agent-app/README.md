# Agent Application with Tool Calling

A FastAPI application demonstrating agentic behavior with tool calling and decision-making, instrumented with Argus SDK.

## Architecture

The agent demonstrates instrumentation at:

1. **L8 (Agents) - Planning**: Agent creates a plan for the task
2. **L8 (Agents) - Tool Selection**: Agent selects the best tool
3. **L8 (Agents) - Tool Execution**: Agent executes the selected tool
4. **L9 (API Gateway)**: Final decision and response

## Signals Emitted

### L8_AGENTS - Reasoning
- **Category**: `reasoning.planning`
- **Captures**: Planning steps, strategy selection
- **Context**: Task breakdown, tool requirements

### L8_AGENTS - Tool Selection
- **Category**: `decision.tool_selection`
- **Captures**: Tool selection logic
- **Context**: Selected tool, arguments, confidence

### L8_AGENTS - Tool Execution
- **Category**: `tool_call.execution`
- **Captures**: Tool invocation and execution
- **Context**: Tool name, arguments, results, latency

## Available Tools

The agent has access to:

1. **search**: Search the knowledge base
2. **calculate**: Perform mathematical calculations
3. **lookup**: Look up information

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

The app will be available at `http://localhost:8001`

## Usage

### Run Agent on Task

```bash
curl -X POST http://localhost:8001/run-agent \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Look up information about Argus",
    "max_steps": 5
  }'
```

### List Available Tools

```bash
curl http://localhost:8001/tools
```

### Health Check

```bash
curl http://localhost:8001/health
```

## Observing Signals

Once the app is running and you've made requests:

1. Open the Argus dashboard at `http://localhost:3000`
2. Go to **Signal Stream**
3. Filter by `app_id = "agent-app"`
4. You should see signals from:
   - L8_AGENTS planning
   - L8_AGENTS tool selection
   - L8_AGENTS tool execution

## Performance Metrics

Each signal includes:

- **Duration**: Time spent in each agent step (ms)
- **Trace ID**: Links all signals in a single agent run
- **Context**: Step details, tool selections, results

## Extending

To add new tools:

1. Add tool to `TOOLS` dict
2. Add handling in `ToolExecutor.execute_tool()`
3. Update agent logic to select and use the tool

Example:

```python
TOOLS = {
    "custom_tool": "Do custom work",
}

@observe(layer=Layer.L8_AGENTS, category="tool_call.execution", client=argus_client)
def execute_tool(self, tool_name: str, args: Dict[str, Any]) -> str:
    if tool_name == "custom_tool":
        return "Custom result"
```
