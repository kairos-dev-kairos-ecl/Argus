"""
Tests for @argus.observe() decorator
"""

import asyncio
import pytest
import time
from unittest.mock import AsyncMock, MagicMock

from sdk.client import ArgusClient, Layer, Severity
from sdk.decorator import observe, ArgusObserver


@pytest.mark.asyncio
async def test_decorator_async_function():
    """Test decorator on async function"""
    mock_client = AsyncMock(spec=ArgusClient)
    mock_client.emit_signal = AsyncMock(return_value=True)

    @observe(layer=Layer.L5_OUTPUT_DECODING, category="inference.test", client=mock_client)
    async def test_func():
        await asyncio.sleep(0.01)  # Simulate work
        return "result"

    result = await test_func()
    assert result == "result"
    assert mock_client.emit_signal.called


@pytest.mark.asyncio
async def test_decorator_fail_open():
    """Test decorator fails open (drops signal when client unavailable)"""
    @observe(layer=Layer.L5_OUTPUT_DECODING, category="inference.test", client=None)
    async def test_func():
        await asyncio.sleep(0.01)
        return "result"

    result = await test_func()
    assert result == "result"


@pytest.mark.asyncio
async def test_decorator_with_exception():
    """Test decorator emits signal on exception"""
    mock_client = AsyncMock(spec=ArgusClient)
    mock_client.emit_signal = AsyncMock(return_value=True)

    @observe(layer=Layer.L5_OUTPUT_DECODING, category="inference.test", client=mock_client)
    async def test_func():
        raise ValueError("Test error")

    with pytest.raises(ValueError):
        await test_func()

    # Signal should still be emitted with error
    assert mock_client.emit_signal.called


@pytest.mark.asyncio
async def test_decorator_measures_duration():
    """Test decorator measures function duration"""
    mock_client = AsyncMock(spec=ArgusClient)
    mock_client.emit_signal = AsyncMock(return_value=True)

    @observe(layer=Layer.L5_OUTPUT_DECODING, category="inference.test", client=mock_client)
    async def test_func():
        await asyncio.sleep(0.05)  # 50ms
        return "result"

    await test_func()

    # Check that duration_ms was passed to emit_signal
    call_kwargs = mock_client.emit_signal.call_args[1]
    assert "duration_ms" in call_kwargs
    # Should be approximately 50ms (allow 20ms variance)
    assert 30 < call_kwargs["duration_ms"] < 100


def test_decorator_sync_function():
    """Test decorator on sync function"""
    # Note: sync decorator uses fire-and-forget for signal emission
    @observe(layer=Layer.L5_OUTPUT_DECODING, category="inference.test", client=None)
    def test_func():
        time.sleep(0.01)
        return "result"

    result = test_func()
    assert result == "result"


@pytest.mark.asyncio
async def test_observer_custom_severity():
    """Test observer with custom severity"""
    mock_client = AsyncMock(spec=ArgusClient)
    mock_client.emit_signal = AsyncMock(return_value=True)

    observer = ArgusObserver(
        layer=Layer.L5_OUTPUT_DECODING,
        category="inference.test",
        severity=Severity.CRITICAL,
        client=mock_client,
    )

    @observer
    async def test_func():
        return "result"

    await test_func()
    assert mock_client.emit_signal.called


@pytest.mark.asyncio
async def test_trace_id_propagation():
    """Test trace ID is propagated"""
    mock_client = AsyncMock(spec=ArgusClient)
    mock_client.emit_signal = AsyncMock(return_value=True)

    @observe(layer=Layer.L5_OUTPUT_DECODING, category="inference.test", client=mock_client)
    async def test_func(_trace_id=None):
        return "result"

    await test_func(_trace_id="test-trace-123")

    # Check that trace_id was passed to emit_signal
    call_kwargs = mock_client.emit_signal.call_args[1]
    assert call_kwargs.get("trace_id") == "test-trace-123"
