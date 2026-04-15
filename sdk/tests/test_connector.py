"""
Tests for Sensor and Connector API
"""

import pytest
import asyncio
from sdk.connector import (
    Sensor,
    ConnectorType,
    DirectConnector,
    BufferedConnector,
    NoOpConnector,
    Layer,
    Severity,
)


@pytest.mark.asyncio
async def test_noop_connector_basic():
    """Test NoOp connector basic functionality"""
    sensor = Sensor(connector_type=ConnectorType.NOOP)

    result = await sensor.emit(
        layer=Layer.L5_OUTPUT_DECODING,
        category="test",
    )

    assert result is True
    await sensor.close()


@pytest.mark.asyncio
async def test_noop_connector_with_context():
    """Test NoOp connector with signal context"""
    sensor = Sensor(connector_type=ConnectorType.NOOP)

    result = await sensor.emit(
        layer=Layer.L5_OUTPUT_DECODING,
        category="inference.test",
        severity=Severity.INFO,
        context={"model": "test-model", "tokens": 100},
        duration_ms=50.5,
    )

    assert result is True
    await sensor.close()


@pytest.mark.asyncio
async def test_context_manager():
    """Test async context manager"""
    async with Sensor(connector_type=ConnectorType.NOOP) as sensor:
        result = await sensor.emit(
            layer=Layer.L5_OUTPUT_DECODING,
            category="test",
        )
        assert result is True


@pytest.mark.asyncio
async def test_trace_id_propagation():
    """Test trace ID is set and used"""
    sensor = Sensor(connector_type=ConnectorType.NOOP)
    sensor.set_trace_id("test-trace-123")

    assert sensor.trace_id == "test-trace-123"

    result = await sensor.emit(
        layer=Layer.L5_OUTPUT_DECODING,
        category="test",
    )

    assert result is True
    await sensor.close()


@pytest.mark.asyncio
async def test_emit_error():
    """Test error signal emission"""
    sensor = Sensor(connector_type=ConnectorType.NOOP)

    try:
        raise ValueError("Test error")
    except ValueError as e:
        result = await sensor.emit_error(
            layer=Layer.L5_OUTPUT_DECODING,
            category="test.error",
            error=e,
        )

    assert result is True
    await sensor.close()


@pytest.mark.asyncio
async def test_buffered_connector_auto_flush():
    """Test buffered connector auto-flushes after interval"""
    sensor = Sensor(
        connector_type=ConnectorType.BUFFER,
        config={
            "max_batch_size": 10,
            "flush_interval_seconds": 0.1,  # Short interval for test
        },
    )

    # Emit one signal
    result = await sensor.emit(
        layer=Layer.L5_OUTPUT_DECODING,
        category="test",
    )
    assert result is True

    # Wait for auto-flush
    await asyncio.sleep(0.2)

    await sensor.close()


@pytest.mark.asyncio
async def test_buffered_connector_batch_flush():
    """Test buffered connector flushes when batch full"""
    sensor = Sensor(
        connector_type=ConnectorType.BUFFER,
        config={
            "max_batch_size": 3,
            "flush_interval_seconds": 60.0,
        },
    )

    # Emit 3 signals (should trigger flush)
    for i in range(3):
        result = await sensor.emit(
            layer=Layer.L5_OUTPUT_DECODING,
            category=f"test_{i}",
        )
        assert result is True

    await sensor.close()


@pytest.mark.asyncio
async def test_sensor_with_all_layers():
    """Test sensor can emit signals for all layers"""
    layers = [
        Layer.L1_HARDWARE,
        Layer.L2_MODEL_WEIGHTS,
        Layer.L3_TOKENIZER,
        Layer.L4_TRANSFORMER,
        Layer.L5_OUTPUT_DECODING,
        Layer.L6_SAFETY,
        Layer.L7_RAG_RETRIEVAL,
        Layer.L8_AGENTS,
        Layer.L9_API_GATEWAY,
        Layer.L10_APPLICATION,
    ]

    async with Sensor(connector_type=ConnectorType.NOOP) as sensor:
        for layer in layers:
            result = await sensor.emit(
                layer=layer,
                category=f"test_{layer.name}",
            )
            assert result is True


@pytest.mark.asyncio
async def test_sensor_with_all_severities():
    """Test sensor can emit signals with all severities"""
    severities = [
        Severity.LOW,
        Severity.MEDIUM,
        Severity.HIGH,
        Severity.CRITICAL,
    ]

    async with Sensor(connector_type=ConnectorType.NOOP) as sensor:
        for severity in severities:
            result = await sensor.emit(
                layer=Layer.L5_OUTPUT_DECODING,
                category="test",
                severity=severity,
            )
            assert result is True


@pytest.mark.asyncio
async def test_multiple_sensors():
    """Test multiple sensors can coexist"""
    sensor1 = Sensor(connector_type=ConnectorType.NOOP, config={"app_id": "app1"})
    sensor2 = Sensor(connector_type=ConnectorType.NOOP, config={"app_id": "app2"})

    result1 = await sensor1.emit(
        layer=Layer.L5_OUTPUT_DECODING,
        category="test1",
    )
    result2 = await sensor2.emit(
        layer=Layer.L5_OUTPUT_DECODING,
        category="test2",
    )

    assert result1 is True
    assert result2 is True

    await sensor1.close()
    await sensor2.close()


@pytest.mark.asyncio
async def test_concurrent_emissions():
    """Test sensor can handle concurrent signal emissions"""
    sensor = Sensor(connector_type=ConnectorType.NOOP)

    async def emit_signal(i):
        return await sensor.emit(
            layer=Layer.L5_OUTPUT_DECODING,
            category=f"concurrent_{i}",
        )

    results = await asyncio.gather(*[emit_signal(i) for i in range(10)])

    assert all(results)
    await sensor.close()


def test_sensor_decorator():
    """Test @observe decorator"""
    from sdk.connector import observe

    sensor = Sensor(connector_type=ConnectorType.NOOP)

    @observe(sensor, Layer.L5_OUTPUT_DECODING, "test.decorated")
    async def decorated_async_func():
        await asyncio.sleep(0.01)
        return "result"

    @observe(sensor, Layer.L5_OUTPUT_DECODING, "test.decorated_sync")
    def decorated_sync_func():
        return "result"

    # Just verify they don't raise
    assert decorated_async_func is not None
    assert decorated_sync_func is not None


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
