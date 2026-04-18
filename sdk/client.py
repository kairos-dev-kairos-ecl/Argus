"""
ArgusClient — Emit signals to Argus for detection and monitoring
"""

import asyncio
import time
import uuid
from datetime import datetime, timezone
from typing import Any, Dict, Optional
from dataclasses import dataclass, asdict
from enum import Enum

import httpx
from google.protobuf.timestamp_pb2 import Timestamp

from .gen.argus.v1 import signal_pb2


class Layer(Enum):
    """10-layer LLM system taxonomy"""
    L1_HARDWARE = 1
    L2_MODEL_WEIGHTS = 2
    L3_TOKENIZER = 3
    L4_TRANSFORMER = 4
    L5_OUTPUT_DECODING = 5
    L6_SAFETY = 6
    L7_RAG_RETRIEVAL = 7
    L8_AGENTS = 8
    L9_API_GATEWAY = 9
    L10_APPLICATION = 10


class Severity(Enum):
    """Signal severity levels"""
    INFO = 1
    LOW = 2
    MEDIUM = 3
    HIGH = 4
    CRITICAL = 5


@dataclass
class SignalContext:
    """Base context for signal emission"""
    layer: Layer
    category: str
    severity: Severity = Severity.INFO
    duration_ms: Optional[float] = None
    trace_id: Optional[str] = None
    parent_span_id: Optional[str] = None


class ArgusClient:
    """
    Async HTTP client for emitting signals to Argus.

    Usage:
        async with ArgusClient("http://localhost:8080") as client:
            await client.emit_signal(
                layer=Layer.L5_OUTPUT_DECODING,
                category="inference.completion",
                severity=Severity.HIGH,
                context={...}
            )
    """

    def __init__(
        self,
        base_url: str = "http://localhost:8080",
        app_id: str = "test-harness",
        app_version: str = "0.1.0",
        sdk_version: str = "0.1.0",
        environment: str = "test",
        timeout: float = 30.0,
        api_key: Optional[str] = None,
    ):
        self.base_url = base_url.rstrip("/")
        self.app_id = app_id
        self.app_version = app_version
        self.sdk_version = sdk_version
        self.environment = environment
        self.timeout = timeout
        self.api_key = api_key
        self.instance_id = str(uuid.uuid4())[:8]
        self.session: Optional[httpx.AsyncClient] = None
        self._trace_id: Optional[str] = None

    async def __aenter__(self):
        self.session = httpx.AsyncClient(timeout=self.timeout)
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        if self.session:
            await self.session.aclose()

    def set_trace_id(self, trace_id: str):
        """Set trace ID for subsequent signals"""
        self._trace_id = trace_id

    def get_trace_id(self) -> str:
        """Get or generate trace ID"""
        if not self._trace_id:
            self._trace_id = str(uuid.uuid4())
        return self._trace_id

    async def emit_signal(
        self,
        layer: Layer,
        category: str,
        severity: Severity = Severity.INFO,
        context: Optional[Dict[str, Any]] = None,
        duration_ms: Optional[float] = None,
        trace_id: Optional[str] = None,
        parent_span_id: Optional[str] = None,
    ) -> bool:
        """
        Emit a signal to Argus.

        Returns:
            True if signal was accepted, False otherwise
        """
        if not self.session:
            raise RuntimeError("Client not initialized. Use 'async with' context manager.")

        # Build signal
        signal_id = self._ulid()
        trace_id = trace_id or self.get_trace_id()
        span_id = str(uuid.uuid4())[:8]

        now = datetime.now(timezone.utc)
        timestamp = Timestamp()
        timestamp.FromDatetime(now)

        signal = signal_pb2.ArgusSignal(
            signal_id=signal_id,
            trace_id=trace_id,
            span_id=span_id,
            parent_span_id=parent_span_id or "",
            layer=layer.value,
            category=category,
            severity=severity.value,
            timestamp=timestamp,
            duration_ms=duration_ms or 0.0,
        )

        # Add source info
        source = signal_pb2.Source(
            app_id=self.app_id,
            app_version=self.app_version,
            sdk_version=self.sdk_version,
            environment=self.environment,
            instance_id=self.instance_id,
        )
        signal.source.CopyFrom(source)

        # Add context (layer-specific)
        if context:
            self._set_context(signal, layer, context)

        # Serialize and emit
        payload = signal.SerializeToString()

        try:
            headers = {"Content-Type": "application/protobuf"}
            if self.api_key:
                headers["Authorization"] = f"Bearer {self.api_key}"

            response = await self.session.post(
                f"{self.base_url}/v1/signals",
                content=payload,
                headers=headers,
            )
            return response.status_code in (200, 201, 202)
        except Exception as e:
            print(f"Error emitting signal: {e}")
            return False

    async def emit_signal_json(
        self,
        layer: Layer,
        category: str,
        severity: Severity = Severity.INFO,
        context: Optional[Dict[str, Any]] = None,
        duration_ms: Optional[float] = None,
        trace_id: Optional[str] = None,
        parent_span_id: Optional[str] = None,
    ) -> bool:
        """
        Emit a signal as JSON (for testing/debugging).
        """
        if not self.session:
            raise RuntimeError("Client not initialized. Use 'async with' context manager.")

        signal_id = self._ulid()
        trace_id = trace_id or self.get_trace_id()
        span_id = str(uuid.uuid4())[:8]

        now = datetime.now(timezone.utc)
        timestamp = Timestamp()
        timestamp.FromDatetime(now)

        signal = signal_pb2.ArgusSignal(
            signal_id=signal_id,
            trace_id=trace_id,
            span_id=span_id,
            parent_span_id=parent_span_id or "",
            layer=layer.value,
            category=category,
            severity=severity.value,
            timestamp=timestamp,
            duration_ms=duration_ms or 0.0,
        )

        source = signal_pb2.Source(
            app_id=self.app_id,
            app_version=self.app_version,
            sdk_version=self.sdk_version,
            environment=self.environment,
            instance_id=self.instance_id,
        )
        signal.source.CopyFrom(source)

        if context:
            self._set_context(signal, layer, context)

        # Convert to JSON
        from google.protobuf.json_format import MessageToJson
        payload_json = MessageToJson(signal)

        try:
            headers = {"Content-Type": "application/json"}
            if self.api_key:
                headers["Authorization"] = f"Bearer {self.api_key}"

            response = await self.session.post(
                f"{self.base_url}/v1/signals",
                content=payload_json,
                headers=headers,
            )
            return response.status_code in (200, 201, 202)
        except Exception as e:
            print(f"Error emitting signal: {e}")
            return False

    def _set_context(self, signal: signal_pb2.ArgusSignal, layer: Layer, context: Dict[str, Any]):
        """Set layer-specific context on signal"""
        if layer == Layer.L3_TOKENIZER:
            ctx = signal_pb2.ContextL3(placeholder=context.get("placeholder", ""))
            signal.context_l3.CopyFrom(ctx)
        elif layer == Layer.L5_OUTPUT_DECODING:
            ctx = signal_pb2.ContextL5(
                operation=context.get("operation", 1),
                output_tokens=context.get("output_tokens", 0),
                input_tokens=context.get("input_tokens", 0),
                total_tokens=context.get("total_tokens", 0),
                finish_reason=context.get("finish_reason", ""),
                temperature=context.get("temperature", 0.7),
                top_p=context.get("top_p", 1.0),
                ttft_ms=context.get("ttft_ms", 0.0),
                tps=context.get("tps", 0.0),
            )
            signal.context_l5.CopyFrom(ctx)
        elif layer == Layer.L6_SAFETY:
            ctx = signal_pb2.ContextL6(placeholder=context.get("placeholder", ""))
            signal.context_l6.CopyFrom(ctx)
        elif layer == Layer.L7_RAG_RETRIEVAL:
            ctx = signal_pb2.ContextL7(
                operation=context.get("operation", 1),
                query_text=context.get("query_text", ""),
                results_count=context.get("results_count", 0),
                embedding_model=context.get("embedding_model", ""),
                vector_index=context.get("vector_index", ""),
                context_window_pct=context.get("context_window_pct", 0.0),
            )
            signal.context_l7.CopyFrom(ctx)
        elif layer == Layer.L8_AGENTS:
            ctx = signal_pb2.ContextL8(
                operation=context.get("operation", 1),
                tool_name=context.get("tool_name", ""),
                tool_provider=context.get("tool_provider", ""),
                tool_result=context.get("tool_result", ""),
                tool_error=context.get("tool_error", ""),
                tool_latency_ms=context.get("tool_latency_ms", 0.0),
                step_number=context.get("step_number", 0),
                total_steps=context.get("total_steps", 0),
                data_flow_tags=context.get("data_flow_tags", []),
                permissions_used=context.get("permissions_used", []),
                permissions_requested=context.get("permissions_requested", []),
            )
            if "tool_arguments" in context:
                ctx.tool_arguments.update(context["tool_arguments"])
            signal.context_l8.CopyFrom(ctx)
        elif layer == Layer.L10_APPLICATION:
            ctx = signal_pb2.ContextL10(placeholder=context.get("placeholder", ""))
            signal.context_l10.CopyFrom(ctx)

    @staticmethod
    def _ulid() -> str:
        """Generate a ULID-like unique ID"""
        import time
        timestamp_ms = int(time.time() * 1000)
        random_part = uuid.uuid4().hex[:12].upper()
        return f"{timestamp_ms:010d}{random_part}"


async def main():
    """Test the client"""
    async with ArgusClient() as client:
        success = await client.emit_signal(
            layer=Layer.L5_OUTPUT_DECODING,
            category="inference.completion",
            severity=Severity.HIGH,
            context={
                "operation": 1,
                "output_tokens": 50,
                "input_tokens": 20,
                "total_tokens": 70,
                "finish_reason": "stop",
            }
        )
        print(f"Signal emitted: {success}")


if __name__ == "__main__":
    asyncio.run(main())
