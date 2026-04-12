"""
SignalBuilder — Construct ArgusSignal messages from context

This module provides utilities for building ArgusSignal protobuf messages
from application context, metrics, and span data.
"""

import uuid
from datetime import datetime, timezone
from typing import Any, Dict, Optional

from google.protobuf.timestamp_pb2 import Timestamp
from .gen.argus.v1 import signal_pb2


class SignalBuilder:
    """Build ArgusSignal messages from context"""

    @staticmethod
    def build(
        layer: int,
        category: str,
        app_id: str,
        app_version: str,
        sdk_version: str,
        environment: str,
        instance_id: str,
        severity: int = 1,
        duration_ms: Optional[float] = None,
        trace_id: Optional[str] = None,
        span_id: Optional[str] = None,
        parent_span_id: Optional[str] = None,
        context: Optional[Dict[str, Any]] = None,
    ) -> signal_pb2.ArgusSignal:
        """
        Build an ArgusSignal message.

        Args:
            layer: Layer enum value (1-10)
            category: Signal category (e.g., "retrieval.search")
            app_id: Application identifier
            app_version: Application version
            sdk_version: SDK version
            environment: Environment (dev|staging|prod)
            instance_id: Instance identifier
            severity: Severity level (1-5)
            duration_ms: Operation duration
            trace_id: Distributed trace ID
            span_id: Span ID
            parent_span_id: Parent span ID
            context: Layer-specific context

        Returns:
            ArgusSignal protobuf message
        """
        signal_id = SignalBuilder._ulid()
        trace_id = trace_id or str(uuid.uuid4())
        span_id = span_id or str(uuid.uuid4())[:8]

        now = datetime.now(timezone.utc)
        timestamp = Timestamp()
        timestamp.FromDatetime(now)

        signal = signal_pb2.ArgusSignal(
            signal_id=signal_id,
            trace_id=trace_id,
            span_id=span_id,
            parent_span_id=parent_span_id or "",
            layer=layer,
            category=category,
            severity=severity,
            timestamp=timestamp,
            duration_ms=duration_ms or 0.0,
        )

        # Add source
        source = signal_pb2.Source(
            app_id=app_id,
            app_version=app_version,
            sdk_version=sdk_version,
            environment=environment,
            instance_id=instance_id,
        )
        signal.source.CopyFrom(source)

        # Add context if provided
        if context:
            SignalBuilder._set_context(signal, layer, context)

        return signal

    @staticmethod
    def _set_context(
        signal: signal_pb2.ArgusSignal, layer: int, context: Dict[str, Any]
    ):
        """Set layer-specific context on signal"""
        if layer == 3:  # L3_TOKENIZER
            ctx = signal_pb2.ContextL3(placeholder=context.get("placeholder", ""))
            signal.context_l3.CopyFrom(ctx)
        elif layer == 5:  # L5_OUTPUT_DECODING
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
        elif layer == 6:  # L6_SAFETY
            ctx = signal_pb2.ContextL6(placeholder=context.get("placeholder", ""))
            signal.context_l6.CopyFrom(ctx)
        elif layer == 7:  # L7_RAG_RETRIEVAL
            ctx = signal_pb2.ContextL7(
                operation=context.get("operation", 1),
                query_text=context.get("query_text", ""),
                results_count=context.get("results_count", 0),
                embedding_model=context.get("embedding_model", ""),
                vector_index=context.get("vector_index", ""),
                context_window_pct=context.get("context_window_pct", 0.0),
            )
            signal.context_l7.CopyFrom(ctx)
        elif layer == 8:  # L8_AGENTS
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
        elif layer == 10:  # L10_APPLICATION
            ctx = signal_pb2.ContextL10(placeholder=context.get("placeholder", ""))
            signal.context_l10.CopyFrom(ctx)

    @staticmethod
    def _ulid() -> str:
        """Generate a ULID-like unique ID"""
        import time
        timestamp_ms = int(time.time() * 1000)
        random_part = uuid.uuid4().hex[:12].upper()
        return f"{timestamp_ms:010d}{random_part}"
