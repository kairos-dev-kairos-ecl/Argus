"""
@argus.observe() Decorator for automatic signal emission

Usage:
    @argus.observe(layer=Layer.L7_RAG_RETRIEVAL, category="retrieval.search")
    async def search_documents(query: str) -> List[str]:
        ...
"""

import asyncio
import functools
import time
import uuid
from typing import Any, Callable, Optional
from datetime import datetime, timezone

from .client import ArgusClient, Layer, Severity
from .signal_builder import SignalBuilder


class ArgusObserver:
    """Decorator for automatic signal emission"""

    def __init__(
        self,
        layer: Layer,
        category: str,
        severity: Severity = Severity.INFO,
        client: Optional[ArgusClient] = None,
        buffer_enabled: bool = True,
        max_buffer_size: int = 100,
    ):
        self.layer = layer
        self.category = category
        self.severity = severity
        self.client = client
        self.buffer_enabled = buffer_enabled
        self.max_buffer_size = max_buffer_size
        self.builder = SignalBuilder()

    def __call__(self, func: Callable) -> Callable:
        """Decorate a function for automatic signal emission"""
        if asyncio.iscoroutinefunction(func):
            return self._wrap_async(func)
        else:
            return self._wrap_sync(func)

    def _wrap_async(self, func: Callable) -> Callable:
        """Wrap an async function"""
        @functools.wraps(func)
        async def wrapper(*args, **kwargs):
            start_time = time.time()
            trace_id = kwargs.pop("_trace_id", None) or str(uuid.uuid4())

            try:
                result = await func(*args, **kwargs)
                duration_ms = (time.time() - start_time) * 1000

                # Emit signal
                await self._emit_signal(
                    duration_ms=duration_ms,
                    trace_id=trace_id,
                    success=True,
                    result=result,
                )
                return result
            except Exception as e:
                duration_ms = (time.time() - start_time) * 1000
                await self._emit_signal(
                    duration_ms=duration_ms,
                    trace_id=trace_id,
                    success=False,
                    error=str(e),
                )
                raise

        return wrapper

    def _wrap_sync(self, func: Callable) -> Callable:
        """Wrap a sync function"""
        @functools.wraps(func)
        def wrapper(*args, **kwargs):
            start_time = time.time()
            trace_id = kwargs.pop("_trace_id", None) or str(uuid.uuid4())

            try:
                result = func(*args, **kwargs)
                duration_ms = (time.time() - start_time) * 1000

                # Emit signal (fire and forget)
                self._emit_signal_sync(
                    duration_ms=duration_ms,
                    trace_id=trace_id,
                    success=True,
                    result=result,
                )
                return result
            except Exception as e:
                duration_ms = (time.time() - start_time) * 1000
                self._emit_signal_sync(
                    duration_ms=duration_ms,
                    trace_id=trace_id,
                    success=False,
                    error=str(e),
                )
                raise

        return wrapper

    async def _emit_signal(
        self,
        duration_ms: float,
        trace_id: str,
        success: bool,
        result: Any = None,
        error: str = None,
    ):
        """Emit a signal (async)"""
        if not self.client:
            return  # Fail-open: drop signal if no client

        context = {}
        if error:
            context["error"] = error

        try:
            await self.client.emit_signal(
                layer=self.layer,
                category=self.category,
                severity=Severity.HIGH if error else self.severity,
                duration_ms=duration_ms,
                trace_id=trace_id,
                context=context,
            )
        except Exception as e:
            # Fail-open: log but don't raise
            print(f"Failed to emit signal: {e}")

    def _emit_signal_sync(
        self,
        duration_ms: float,
        trace_id: str,
        success: bool,
        result: Any = None,
        error: str = None,
    ):
        """Emit a signal (sync, fire and forget)"""
        if not self.client:
            return  # Fail-open: drop signal if no client

        # Fire and forget in background
        try:
            loop = asyncio.get_event_loop()
        except RuntimeError:
            loop = asyncio.new_event_loop()
            asyncio.set_event_loop(loop)

        asyncio.create_task(
            self._emit_signal(
                duration_ms=duration_ms,
                trace_id=trace_id,
                success=success,
                result=result,
                error=error,
            )
        )


def observe(
    layer: Layer,
    category: str,
    severity: Severity = Severity.INFO,
    client: Optional[ArgusClient] = None,
):
    """
    Decorator for automatic signal emission.

    Usage:
        @observe(layer=Layer.L7_RAG_RETRIEVAL, category="retrieval.search")
        async def search_documents(query: str) -> List[str]:
            ...
    """
    return ArgusObserver(
        layer=layer,
        category=category,
        severity=severity,
        client=client,
    )
