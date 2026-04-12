"""
Buffer — In-memory signal buffer with drop counter

Provides buffered signal emission with automatic batching and drop counter
for fail-open behavior when Argus is unreachable.
"""

import asyncio
import time
from collections import deque
from typing import Optional
from dataclasses import dataclass

from .gen.argus.v1 import signal_pb2


@dataclass
class BufferStats:
    """Statistics about buffer operations"""
    total_emitted: int = 0
    total_dropped: int = 0
    buffer_size: int = 0
    last_flush: Optional[float] = None


class SignalBuffer:
    """
    In-memory buffer for signals with automatic batching.

    Provides:
    - Buffering up to max_size signals
    - Drop counter when buffer is full
    - Async flush operations
    - Fail-open behavior (increments drop counter when Argus unreachable)
    """

    def __init__(
        self,
        max_size: int = 100,
        flush_interval_seconds: float = 1.0,
    ):
        self.max_size = max_size
        self.flush_interval_seconds = flush_interval_seconds
        self.buffer: deque[signal_pb2.ArgusSignal] = deque()
        self.drop_counter = 0
        self.stats = BufferStats()
        self._lock = asyncio.Lock()
        self._flush_task: Optional[asyncio.Task] = None

    async def add(self, signal: signal_pb2.ArgusSignal) -> bool:
        """
        Add a signal to the buffer.

        Returns:
            True if signal was added, False if buffer is full (signal dropped)
        """
        async with self._lock:
            if len(self.buffer) >= self.max_size:
                # Buffer is full, drop signal
                self.drop_counter += 1
                self.stats.total_dropped += 1
                return False

            self.buffer.append(signal)
            self.stats.total_emitted += 1
            return True

    async def flush(self, emit_fn) -> int:
        """
        Flush buffered signals using the provided emit function.

        Args:
            emit_fn: Async function to emit signals (receives list of signals)

        Returns:
            Number of signals flushed
        """
        async with self._lock:
            if not self.buffer:
                return 0

            # Get all buffered signals
            signals = list(self.buffer)
            self.buffer.clear()

            try:
                await emit_fn(signals)
                self.stats.last_flush = time.time()
                return len(signals)
            except Exception as e:
                # Re-add signals on failure (fail-open)
                for signal in signals:
                    try:
                        self.buffer.append(signal)
                    except IndexError:
                        self.drop_counter += 1
                        self.stats.total_dropped += 1
                return 0

    async def start_auto_flush(self, emit_fn):
        """
        Start automatic flush task.

        Args:
            emit_fn: Async function to emit signals
        """
        async def auto_flush():
            while True:
                try:
                    await asyncio.sleep(self.flush_interval_seconds)
                    await self.flush(emit_fn)
                except asyncio.CancelledError:
                    break
                except Exception as e:
                    print(f"Error in auto flush: {e}")

        self._flush_task = asyncio.create_task(auto_flush())

    async def stop_auto_flush(self):
        """Stop automatic flush task"""
        if self._flush_task:
            self._flush_task.cancel()
            try:
                await self._flush_task
            except asyncio.CancelledError:
                pass

    def get_drop_counter(self) -> int:
        """Get the current drop counter"""
        return self.drop_counter

    def get_stats(self) -> BufferStats:
        """Get buffer statistics"""
        self.stats.buffer_size = len(self.buffer)
        return self.stats

    async def emit_drop_counter_signal(
        self,
        emit_fn,
        app_id: str,
        app_version: str,
        sdk_version: str,
        environment: str,
        instance_id: str,
    ):
        """
        Emit a signal indicating drops (fail-open behavior).

        When Argus is unreachable, emit a drop counter signal periodically
        so operators know the SDK is dropping signals.
        """
        if self.drop_counter == 0:
            return

        from .gen.argus.v1 import signal_pb2
        import uuid
        from datetime import datetime, timezone
        from google.protobuf.timestamp_pb2 import Timestamp

        signal_id = f"{int(time.time() * 1000):010d}{uuid.uuid4().hex[:12].upper()}"
        now = datetime.now(timezone.utc)
        timestamp = Timestamp()
        timestamp.FromDatetime(now)

        # Create a special drop-counter signal
        signal = signal_pb2.ArgusSignal(
            signal_id=signal_id,
            trace_id=f"drop-counter-{int(time.time())}",
            span_id="drop-counter",
            layer=10,  # L10_APPLICATION
            category="sdk.drop_counter",
            severity=4,  # HIGH
            timestamp=timestamp,
            duration_ms=0.0,
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

        # Add context with drop count
        ctx = signal_pb2.ContextL10(placeholder=f"dropped_signals={self.drop_counter}")
        signal.context_l10.CopyFrom(ctx)

        try:
            await emit_fn([signal])
            self.drop_counter = 0  # Reset after successful emit
        except Exception as e:
            print(f"Failed to emit drop counter signal: {e}")
