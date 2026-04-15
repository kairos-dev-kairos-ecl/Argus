"""
Argus Connector Package
High-level API for emitting signals through configurable backends.
Abstracts away signal creation, batching, and routing.
"""

import asyncio
import json
from typing import Any, Dict, Optional, List
from enum import Enum
from datetime import datetime
from abc import ABC, abstractmethod
import logging

from sdk.client import ArgusClient, Layer, Severity
from sdk.signal_builder import SignalBuilder

logger = logging.getLogger(__name__)


class ConnectorType(Enum):
    """Supported connector backends"""
    DIRECT = "direct"           # Direct GRPC to Argus
    HTTP = "http"               # HTTP POST to Argus
    OTLP = "otlp"               # OpenTelemetry Protocol
    BUFFER = "buffer"           # Buffered (batch) mode
    KAFKA = "kafka"             # Kafka topic (future)
    NOOP = "noop"               # No-op for testing


class BaseConnector(ABC):
    """Base connector interface"""

    def __init__(self, config: Dict[str, Any]):
        self.config = config
        self.enabled = True
        self._queue: List[Dict[str, Any]] = []

    @abstractmethod
    async def send(self, signal: Dict[str, Any]) -> bool:
        """Send a signal. Return True if successful."""
        pass

    @abstractmethod
    async def close(self):
        """Cleanup resources"""
        pass


class DirectConnector(BaseConnector):
    """Direct connector using Argus Python SDK"""

    def __init__(self, config: Dict[str, Any]):
        super().__init__(config)
        self.client = ArgusClient(
            base_url=config.get("base_url", "http://localhost:8080"),
            app_id=config.get("app_id", "unknown"),
            app_version=config.get("app_version", "0.1.0"),
            environment=config.get("environment", "dev"),
            timeout=config.get("timeout", 30.0),
        )

    async def send(self, signal: Dict[str, Any]) -> bool:
        """Emit signal via SDK"""
        try:
            success = await self.client.emit_signal(
                layer=signal.get("layer"),
                category=signal.get("category"),
                severity=signal.get("severity", Severity.INFO),
                context=signal.get("context", {}),
                duration_ms=signal.get("duration_ms"),
                trace_id=signal.get("trace_id"),
            )
            return success
        except Exception as e:
            logger.error(f"Failed to emit signal: {e}")
            return False

    async def close(self):
        """Close client"""
        self.client.close()


class BufferedConnector(BaseConnector):
    """Buffered connector - batches signals before sending"""

    def __init__(self, config: Dict[str, Any]):
        super().__init__(config)
        self.max_batch_size = config.get("max_batch_size", 100)
        self.flush_interval_seconds = config.get("flush_interval_seconds", 5.0)
        self.backend = DirectConnector(config)
        self._flush_task = None

    async def send(self, signal: Dict[str, Any]) -> bool:
        """Buffer signal, flush if batch full"""
        self._queue.append(signal)

        if len(self._queue) >= self.max_batch_size:
            await self.flush()
            return True

        # Start flush timer if not running
        if self._flush_task is None or self._flush_task.done():
            self._flush_task = asyncio.create_task(self._auto_flush())

        return True

    async def _auto_flush(self):
        """Auto-flush after interval"""
        await asyncio.sleep(self.flush_interval_seconds)
        await self.flush()

    async def flush(self):
        """Send all buffered signals"""
        if not self._queue:
            return

        batch = self._queue[:]
        self._queue.clear()

        logger.info(f"Flushing {len(batch)} signals")
        for signal in batch:
            try:
                await self.backend.send(signal)
            except Exception as e:
                logger.error(f"Failed to send buffered signal: {e}")

    async def close(self):
        """Flush remaining signals and cleanup"""
        await self.flush()
        await self.backend.close()


class NoOpConnector(BaseConnector):
    """No-op connector for testing"""

    async def send(self, signal: Dict[str, Any]) -> bool:
        logger.debug(f"NoOp: {signal.get('category')}")
        return True

    async def close(self):
        pass


class Sensor:
    """High-level sensor API for emitting signals"""

    def __init__(
        self,
        connector_type: ConnectorType = ConnectorType.DIRECT,
        config: Optional[Dict[str, Any]] = None,
    ):
        self.connector_type = connector_type
        self.config = config or {}
        self.connector = self._create_connector()
        self.trace_id: Optional[str] = None

    def _create_connector(self) -> BaseConnector:
        """Create connector based on type"""
        if self.connector_type == ConnectorType.DIRECT:
            return DirectConnector(self.config)
        elif self.connector_type == ConnectorType.BUFFER:
            return BufferedConnector(self.config)
        elif self.connector_type == ConnectorType.NOOP:
            return NoOpConnector(self.config)
        else:
            raise ValueError(f"Unknown connector type: {self.connector_type}")

    def set_trace_id(self, trace_id: str):
        """Set trace ID for signal correlation"""
        self.trace_id = trace_id

    async def emit(
        self,
        layer: Layer,
        category: str,
        severity: Severity = Severity.INFO,
        context: Optional[Dict[str, Any]] = None,
        duration_ms: Optional[float] = None,
    ) -> bool:
        """Emit a signal"""
        signal = {
            "layer": layer,
            "category": category,
            "severity": severity,
            "context": context or {},
            "duration_ms": duration_ms,
            "trace_id": self.trace_id,
        }
        return await self.connector.send(signal)

    async def emit_error(
        self,
        layer: Layer,
        category: str,
        error: Exception,
        duration_ms: Optional[float] = None,
    ) -> bool:
        """Emit an error signal"""
        return await self.emit(
            layer=layer,
            category=category,
            severity=Severity.HIGH,
            context={"error": str(error), "error_type": type(error).__name__},
            duration_ms=duration_ms,
        )

    async def close(self):
        """Cleanup resources"""
        await self.connector.close()

    async def __aenter__(self):
        """Async context manager entry"""
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        """Async context manager exit"""
        await self.close()


class ObservedFunction:
    """Decorator class for observing functions"""

    def __init__(
        self,
        sensor: Sensor,
        layer: Layer,
        category: str,
        severity: Severity = Severity.INFO,
    ):
        self.sensor = sensor
        self.layer = layer
        self.category = category
        self.severity = severity

    def __call__(self, func):
        """Decorate function"""
        async def async_wrapper(*args, **kwargs):
            import time
            start = time.time()
            try:
                result = await func(*args, **kwargs)
                duration_ms = (time.time() - start) * 1000
                await self.sensor.emit(
                    layer=self.layer,
                    category=self.category,
                    severity=self.severity,
                    duration_ms=duration_ms,
                )
                return result
            except Exception as e:
                duration_ms = (time.time() - start) * 1000
                await self.sensor.emit_error(
                    layer=self.layer,
                    category=self.category,
                    error=e,
                    duration_ms=duration_ms,
                )
                raise

        def sync_wrapper(*args, **kwargs):
            import time
            start = time.time()
            try:
                result = func(*args, **kwargs)
                duration_ms = (time.time() - start) * 1000
                asyncio.create_task(
                    self.sensor.emit(
                        layer=self.layer,
                        category=self.category,
                        severity=self.severity,
                        duration_ms=duration_ms,
                    )
                )
                return result
            except Exception as e:
                duration_ms = (time.time() - start) * 1000
                asyncio.create_task(
                    self.sensor.emit_error(
                        layer=self.layer,
                        category=self.category,
                        error=e,
                        duration_ms=duration_ms,
                    )
                )
                raise

        if asyncio.iscoroutinefunction(func):
            return async_wrapper
        else:
            return sync_wrapper


def observe(
    sensor: Sensor,
    layer: Layer,
    category: str,
    severity: Severity = Severity.INFO,
):
    """Decorator for observing function execution"""
    return ObservedFunction(sensor, layer, category, severity)


# Singleton sensor instance (optional)
_default_sensor: Optional[Sensor] = None


def init_default_sensor(
    connector_type: ConnectorType = ConnectorType.DIRECT,
    config: Optional[Dict[str, Any]] = None,
):
    """Initialize default sensor"""
    global _default_sensor
    _default_sensor = Sensor(connector_type=connector_type, config=config)


def get_default_sensor() -> Sensor:
    """Get default sensor instance"""
    global _default_sensor
    if _default_sensor is None:
        init_default_sensor()
    return _default_sensor
