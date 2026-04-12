"""
OTLPToArgusConverter - Convert OpenTelemetry spans to Argus signals
"""

import json
from typing import Any, Dict, Optional
from datetime import datetime, timezone

# Protobuf imports
from sdk.gen.argus.v1 import signal_pb2
from google.protobuf.timestamp_pb2 import Timestamp

# OTEL span kind mappings to Argus layers
SPAN_KIND_TO_LAYER = {
    0: 10,  # UNSPECIFIED -> L10_APPLICATION
    1: 8,   # INTERNAL -> L8_AGENTS
    2: 7,   # SERVER -> L7_RAG_RETRIEVAL
    3: 9,   # CLIENT -> L9_API_GATEWAY
    4: 5,   # PRODUCER -> L5_OUTPUT_DECODING
    5: 4,   # CONSUMER -> L4_TRANSFORMER
}

# OTEL status codes
STATUS_UNSET = 0
STATUS_OK = 1
STATUS_ERROR = 2

class OTLPToArgusConverter:
    """Convert OTLP spans to Argus signals"""

    @staticmethod
    def convert_span(
        span: Dict[str, Any],
        app_id: str = "otel-app",
        sdk_version: str = "0.1.0",
        default_environment: str = "dev",
    ) -> signal_pb2.ArgusSignal:
        """
        Convert an OTLP span to an ArgusSignal.

        Args:
            span: OTLP span as dict (from protobuf JSON)
            app_id: Application ID
            sdk_version: SDK version
            default_environment: Default environment

        Returns:
            ArgusSignal protobuf message
        """
        # Extract basic span info
        trace_id = span.get("traceId", "")
        span_id = span.get("spanId", "")
        parent_span_id = span.get("parentSpanId", "")
        name = span.get("name", "")
        kind = span.get("kind", 0)
        status = span.get("status", {})

        # Extract timestamps
        start_time_unix_nano = int(span.get("startTimeUnixNano", 0))
        end_time_unix_nano = int(span.get("endTimeUnixNano", 0))

        # Calculate duration in milliseconds
        duration_ms = (end_time_unix_nano - start_time_unix_nano) / 1_000_000 if end_time_unix_nano > start_time_unix_nano else 0

        # Extract attributes
        attributes = {}
        for attr in span.get("attributes", []):
            key = attr.get("key", "")
            value = attr.get("value", {})
            # Extract value from oneof structure
            if "stringValue" in value:
                attributes[key] = value["stringValue"]
            elif "intValue" in value:
                attributes[key] = int(value["intValue"])
            elif "doubleValue" in value:
                attributes[key] = float(value["doubleValue"])
            elif "boolValue" in value:
                attributes[key] = value["boolValue"]

        # Determine layer (from attribute or span kind)
        layer = attributes.get("argus.layer")
        if layer is None:
            layer = SPAN_KIND_TO_LAYER.get(kind, 10)
        else:
            layer = int(layer)

        # Determine category
        category = attributes.get("argus.category", "otel." + name.replace(" ", "_").lower())

        # Determine severity (from status)
        severity = signal_pb2.Severity.INFO
        if status.get("code") == STATUS_ERROR:
            severity = signal_pb2.Severity.HIGH
        elif status.get("code") == STATUS_OK:
            severity = signal_pb2.Severity.INFO

        # Get environment from attributes or default
        environment = attributes.get("deployment.environment", default_environment)

        # Create signal
        signal = signal_pb2.ArgusSignal(
            signal_id=OTLPToArgusConverter._generate_signal_id(),
            trace_id=trace_id,
            span_id=span_id,
            parent_span_id=parent_span_id or "",
            layer=layer,
            category=category,
            severity=severity,
            duration_ms=duration_ms,
        )

        # Set timestamp
        start_time_seconds = start_time_unix_nano // 1_000_000_000
        start_time_nanos = start_time_unix_nano % 1_000_000_000
        signal.timestamp.seconds = start_time_seconds
        signal.timestamp.nanos = start_time_nanos

        # Set source
        source = signal_pb2.Source(
            app_id=attributes.get("service.name", app_id),
            app_version=attributes.get("service.version", "unknown"),
            sdk_version=sdk_version,
            environment=environment,
            instance_id=attributes.get("service.instance.id", "unknown"),
        )
        signal.source.CopyFrom(source)

        # Set context based on layer
        OTLPToArgusConverter._set_context_from_attributes(signal, layer, attributes)

        # Handle status code message
        if "message" in status:
            # Add error message to context
            if layer == 10:
                ctx = signal_pb2.ContextL10(placeholder=status["message"])
                signal.context_l10.CopyFrom(ctx)

        return signal

    @staticmethod
    def _set_context_from_attributes(
        signal: signal_pb2.ArgusSignal,
        layer: int,
        attributes: Dict[str, Any],
    ):
        """Set layer-specific context from span attributes"""
        if layer == 5:  # L5_OUTPUT_DECODING
            ctx = signal_pb2.ContextL5(
                operation=int(attributes.get("llm.operation", 1)),
                input_tokens=int(attributes.get("llm.input_tokens", 0)),
                output_tokens=int(attributes.get("llm.output_tokens", 0)),
                finish_reason=attributes.get("llm.finish_reason", ""),
                temperature=float(attributes.get("llm.temperature", 0.7)),
                top_p=float(attributes.get("llm.top_p", 1.0)),
            )
            signal.context_l5.CopyFrom(ctx)

        elif layer == 7:  # L7_RAG_RETRIEVAL
            ctx = signal_pb2.ContextL7(
                operation=int(attributes.get("retrieval.operation", 1)),
                query_text=attributes.get("retrieval.query", ""),
                results_count=int(attributes.get("retrieval.results_count", 0)),
                embedding_model=attributes.get("retrieval.embedding_model", ""),
                vector_index=attributes.get("retrieval.vector_index", ""),
            )
            signal.context_l7.CopyFrom(ctx)

        elif layer == 8:  # L8_AGENTS
            ctx = signal_pb2.ContextL8(
                operation=int(attributes.get("agent.operation", 1)),
                tool_name=attributes.get("agent.tool_name", ""),
                tool_provider=attributes.get("agent.tool_provider", ""),
                tool_latency_ms=float(attributes.get("agent.tool_latency_ms", 0.0)),
                step_number=int(attributes.get("agent.step_number", 0)),
                total_steps=int(attributes.get("agent.total_steps", 0)),
            )
            signal.context_l8.CopyFrom(ctx)

    @staticmethod
    def _generate_signal_id() -> str:
        """Generate a signal ID"""
        import time
        import uuid
        timestamp_ms = int(time.time() * 1000)
        random_part = uuid.uuid4().hex[:12].upper()
        return f"{timestamp_ms:010d}{random_part}"

    @staticmethod
    def batch_convert_spans(
        spans: list,
        app_id: str = "otel-app",
        sdk_version: str = "0.1.0",
        default_environment: str = "dev",
    ) -> list:
        """
        Convert multiple OTLP spans to ArgusSignals.

        Args:
            spans: List of OTLP spans
            app_id: Application ID
            sdk_version: SDK version
            default_environment: Default environment

        Returns:
            List of ArgusSignal messages
        """
        signals = []
        for span in spans:
            try:
                signal = OTLPToArgusConverter.convert_span(
                    span, app_id, sdk_version, default_environment
                )
                signals.append(signal)
            except Exception as e:
                print(f"Failed to convert span: {e}")

        return signals
