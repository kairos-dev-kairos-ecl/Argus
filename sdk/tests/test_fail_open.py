"""
Tests for fail-open behavior (drop counter when Argus unreachable)
"""

import asyncio
import pytest
from unittest.mock import AsyncMock, patch

from sdk.client import ArgusClient, Layer, Severity
from sdk.buffer import SignalBuffer


@pytest.mark.asyncio
async def test_buffer_drop_counter_increments():
    """Test drop counter increments when buffer is full"""
    buffer = SignalBuffer(max_size=2)

    from sdk.gen.argus.v1 import signal_pb2
    from google.protobuf.timestamp_pb2 import Timestamp
    from datetime import datetime, timezone

    # Create two signals
    for i in range(2):
        signal = signal_pb2.ArgusSignal(
            signal_id=f"signal-{i}",
            trace_id="trace-1",
            span_id=f"span-{i}",
            layer=10,
            category="test",
            severity=1,
        )
        now = datetime.now(timezone.utc)
        timestamp = Timestamp()
        timestamp.FromDatetime(now)
        signal.timestamp.CopyFrom(timestamp)

        await buffer.add(signal)

    # Buffer should be full now
    assert len(buffer.buffer) == 2
    assert buffer.drop_counter == 0

    # Adding third signal should be dropped
    signal = signal_pb2.ArgusSignal(
        signal_id="signal-3",
        trace_id="trace-1",
        span_id="span-3",
        layer=10,
        category="test",
        severity=1,
    )
    now = datetime.now(timezone.utc)
    timestamp = Timestamp()
    timestamp.FromDatetime(now)
    signal.timestamp.CopyFrom(timestamp)

    result = await buffer.add(signal)
    assert result is False
    assert buffer.drop_counter == 1


@pytest.mark.asyncio
async def test_buffer_emit_drop_counter_signal():
    """Test emission of drop counter signal"""
    buffer = SignalBuffer(max_size=1)
    buffer.drop_counter = 5

    emit_called = False
    emitted_signals = []

    async def mock_emit_fn(signals):
        nonlocal emit_called, emitted_signals
        emit_called = True
        emitted_signals = signals

    await buffer.emit_drop_counter_signal(
        mock_emit_fn,
        app_id="test-app",
        app_version="1.0",
        sdk_version="0.1.0",
        environment="test",
        instance_id="test-instance",
    )

    assert emit_called is True
    assert len(emitted_signals) == 1
    assert emitted_signals[0].category == "sdk.drop_counter"
    assert buffer.drop_counter == 0  # Should be reset after emit


@pytest.mark.asyncio
async def test_client_fail_open_on_connection_error():
    """Test that client fails open on connection error"""
    with patch("httpx.AsyncClient.post") as mock_post:
        mock_post.side_effect = ConnectionError("Argus unreachable")

        async with ArgusClient() as client:
            # Should not raise, should fail open
            success = await client.emit_signal(
                layer=Layer.L5_OUTPUT_DECODING,
                category="inference.completion",
            )

        assert success is False


@pytest.mark.asyncio
async def test_decorator_fail_open_no_client():
    """Test decorator with no client (fail-open)"""
    from sdk.decorator import observe

    called = False

    @observe(layer=Layer.L5_OUTPUT_DECODING, category="test", client=None)
    async def test_func():
        nonlocal called
        called = True
        return "result"

    result = await test_func()
    assert result == "result"
    assert called is True


@pytest.mark.asyncio
async def test_decorator_fail_open_emit_error():
    """Test decorator fails open if emit_signal raises"""
    from sdk.decorator import observe

    mock_client = AsyncMock(spec=ArgusClient)
    mock_client.emit_signal = AsyncMock(side_effect=Exception("Argus error"))

    @observe(layer=Layer.L5_OUTPUT_DECODING, category="test", client=mock_client)
    async def test_func():
        return "result"

    # Should not raise
    result = await test_func()
    assert result == "result"


@pytest.mark.asyncio
async def test_buffer_fail_open_re_adds_on_flush_failure():
    """Test buffer re-adds signals on flush failure"""
    buffer = SignalBuffer(max_size=10)

    from sdk.gen.argus.v1 import signal_pb2
    from google.protobuf.timestamp_pb2 import Timestamp
    from datetime import datetime, timezone

    signal = signal_pb2.ArgusSignal(
        signal_id="signal-1",
        trace_id="trace-1",
        span_id="span-1",
        layer=10,
        category="test",
        severity=1,
    )
    now = datetime.now(timezone.utc)
    timestamp = Timestamp()
    timestamp.FromDatetime(now)
    signal.timestamp.CopyFrom(timestamp)

    await buffer.add(signal)
    assert len(buffer.buffer) == 1

    # Flush with failure
    async def failing_emit(signals):
        raise Exception("Argus unreachable")

    flushed = await buffer.flush(failing_emit)
    assert flushed == 0  # No signals emitted

    # Buffer should have re-added the signal
    assert len(buffer.buffer) == 1


@pytest.mark.asyncio
async def test_buffer_auto_flush_continues_on_error():
    """Test auto-flush task continues on error"""
    buffer = SignalBuffer(max_size=10, flush_interval_seconds=0.01)

    call_count = 0

    async def failing_emit(signals):
        nonlocal call_count
        call_count += 1
        if call_count == 1:
            raise Exception("First call fails")
        # Second call succeeds

    # Add a signal so there's something to flush
    from sdk.gen.argus.v1 import signal_pb2
    from google.protobuf.timestamp_pb2 import Timestamp
    from datetime import datetime, timezone

    signal = signal_pb2.ArgusSignal(
        signal_id="signal-1",
        trace_id="trace-1",
        span_id="span-1",
        layer=10,
        category="test",
        severity=1,
    )
    now = datetime.now(timezone.utc)
    timestamp = Timestamp()
    timestamp.FromDatetime(now)
    signal.timestamp.CopyFrom(timestamp)
    await buffer.add(signal)

    await buffer.start_auto_flush(failing_emit)

    # Wait for at least 2 flushes
    await asyncio.sleep(0.05)

    await buffer.stop_auto_flush()

    # Should have been called at least twice despite first failure
    assert call_count >= 2
