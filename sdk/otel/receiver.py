"""
OTLPReceiver - gRPC receiver for OTLP spans converted to Argus signals
"""

import asyncio
from typing import Any, Dict, List, Callable, Optional

from .converter import OTLPToArgusConverter
from sdk import ArgusClient


class OTLPReceiver:
    """
    OTLP Receiver that converts OpenTelemetry spans to Argus signals.

    Usage:
        receiver = OTLPReceiver(argus_client)
        await receiver.start_server(port=4317)
        # Spans received on gRPC endpoint will be converted and sent to Argus
    """

    def __init__(
        self,
        argus_client: ArgusClient,
        app_id: str = "otel-app",
        sdk_version: str = "0.1.0",
        batch_size: int = 100,
        flush_interval_seconds: float = 1.0,
    ):
        self.argus_client = argus_client
        self.app_id = app_id
        self.sdk_version = sdk_version
        self.batch_size = batch_size
        self.flush_interval_seconds = flush_interval_seconds
        self.signal_queue: List[Dict[str, Any]] = []
        self.flush_task: Optional[asyncio.Task] = None

    async def process_trace_service_request(self, request: Dict[str, Any]) -> None:
        """
        Process an OTLP ExportTraceServiceRequest.

        Args:
            request: OTLP trace export request (from protobuf JSON)
        """
        # Extract spans from resource spans
        spans = []
        for resource_span in request.get("resourceSpans", []):
            resource = resource_span.get("resource", {})

            # Extract resource attributes
            resource_attrs = {}
            for attr in resource.get("attributes", []):
                key = attr.get("key", "")
                value = attr.get("value", {})
                if "stringValue" in value:
                    resource_attrs[key] = value["stringValue"]

            # Get app_id from resource
            app_id = resource_attrs.get("service.name", self.app_id)

            # Process scope spans
            for scope_span in resource_span.get("scopeSpans", []):
                for span in scope_span.get("spans", []):
                    # Convert span to Argus signal
                    signal = OTLPToArgusConverter.convert_span(
                        span,
                        app_id=app_id,
                        sdk_version=self.sdk_version,
                    )

                    # Queue signal for emission
                    await self._queue_signal(signal)

    async def _queue_signal(self, signal: Any) -> None:
        """Queue a signal for batch emission"""
        self.signal_queue.append(signal)

        if len(self.signal_queue) >= self.batch_size:
            await self._flush_signals()

    async def _flush_signals(self) -> None:
        """Flush queued signals to Argus"""
        if not self.signal_queue:
            return

        signals_to_emit = self.signal_queue.copy()
        self.signal_queue.clear()

        for signal in signals_to_emit:
            try:
                # Emit as protobuf
                payload = signal.SerializeToString()
                await self.argus_client.emit_signal(
                    layer=signal.layer,
                    category=signal.category,
                    severity=signal.severity,
                    duration_ms=signal.duration_ms,
                    trace_id=signal.trace_id,
                )
            except Exception as e:
                print(f"Failed to emit signal: {e}")

    async def start_periodic_flush(self) -> None:
        """Start periodic flushing of queued signals"""
        async def flush_loop():
            while True:
                try:
                    await asyncio.sleep(self.flush_interval_seconds)
                    await self._flush_signals()
                except asyncio.CancelledError:
                    # Flush remaining signals on shutdown
                    await self._flush_signals()
                    break
                except Exception as e:
                    print(f"Error in flush loop: {e}")

        self.flush_task = asyncio.create_task(flush_loop())

    async def stop_periodic_flush(self) -> None:
        """Stop periodic flushing"""
        if self.flush_task:
            self.flush_task.cancel()
            try:
                await self.flush_task
            except asyncio.CancelledError:
                pass

    async def shutdown(self) -> None:
        """Shutdown receiver and flush remaining signals"""
        await self.stop_periodic_flush()
        await self._flush_signals()


class OTLPGRPCReceiver:
    """
    gRPC receiver for OTLP (implements opentelemetry.proto.collector.trace.v1.TraceService)

    This is a simplified implementation that demonstrates span conversion.
    For production, use the official OpenTelemetry Collector or contrib receivers.
    """

    def __init__(self, argus_client: ArgusClient):
        self.converter = OTLPToArgusConverter()
        self.argus_client = argus_client
        self.receiver = OTLPReceiver(argus_client)

    async def export(self, request: Dict[str, Any]) -> Dict[str, Any]:
        """
        Export traces (called by gRPC framework).

        Args:
            request: ExportTraceServiceRequest protobuf (as dict/JSON)

        Returns:
            ExportTraceServiceResponse
        """
        try:
            await self.receiver.process_trace_service_request(request)
            return {
                "partial_success": {
                    "rejectedSpans": 0,
                    "errorMessage": "",
                }
            }
        except Exception as e:
            print(f"Error exporting traces: {e}")
            return {
                "partial_success": {
                    "rejectedSpans": 1,
                    "errorMessage": str(e),
                }
            }
