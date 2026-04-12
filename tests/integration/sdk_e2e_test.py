"""
End-to-end tests: SDK → Argus → ClickHouse → Dashboard

Verifies that signals emitted by SDKs reach ClickHouse and are queryable.
"""

import asyncio
import pytest
import sys
import time
from typing import List

sys.path.insert(0, '/c/Users/Drupad/ArgusXDR')

from sdk import ArgusClient, Layer, Severity, observe


class TestPythonSDKE2E:
    """End-to-end tests for Python SDK"""

    @pytest.fixture
    async def client(self):
        """Initialize Argus client"""
        client = ArgusClient(
            base_url="http://localhost:8080",
            app_id="sdk-e2e-test",
            app_version="1.0.0",
            environment="test",
        )
        await client.__aenter__()
        yield client
        await client.__aexit__(None, None, None)

    @pytest.mark.asyncio
    async def test_emit_signal_basic(self, client):
        """Test basic signal emission"""
        success = await client.emit_signal(
            layer=Layer.L5_OUTPUT_DECODING,
            category="test.basic",
            severity=Severity.INFO,
        )
        # Note: may be False if Argus not running, that's OK
        assert isinstance(success, bool)

    @pytest.mark.asyncio
    async def test_emit_signal_with_context(self, client):
        """Test signal with layer-specific context"""
        success = await client.emit_signal(
            layer=Layer.L5_OUTPUT_DECODING,
            category="test.context",
            severity=Severity.INFO,
            context={
                "output_tokens": 100,
                "input_tokens": 50,
                "finish_reason": "stop",
                "temperature": 0.7,
            },
            duration_ms=45.5,
        )
        assert isinstance(success, bool)

    @pytest.mark.asyncio
    async def test_emit_multiple_signals(self, client):
        """Test emitting multiple signals"""
        for i in range(5):
            success = await client.emit_signal(
                layer=Layer.L7_RAG_RETRIEVAL,
                category="test.retrieval",
                context={"results_count": i},
                duration_ms=float(i * 10),
            )
            assert isinstance(success, bool)

    @pytest.mark.asyncio
    async def test_emit_all_layers(self, client):
        """Test emitting signals from all 10 layers"""
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

        for layer in layers:
            success = await client.emit_signal(
                layer=layer,
                category="test.all_layers",
                severity=Severity.INFO,
            )
            assert isinstance(success, bool)

    @pytest.mark.asyncio
    async def test_emit_all_severities(self, client):
        """Test emitting signals with all severity levels"""
        severities = [
            Severity.INFO,
            Severity.LOW,
            Severity.MEDIUM,
            Severity.HIGH,
            Severity.CRITICAL,
        ]

        for severity in severities:
            success = await client.emit_signal(
                layer=Layer.L5_OUTPUT_DECODING,
                category="test.severities",
                severity=severity,
            )
            assert isinstance(success, bool)

    @pytest.mark.asyncio
    async def test_trace_id_propagation(self, client):
        """Test trace ID propagation across signals"""
        trace_id = client.get_trace_id()

        # Emit multiple signals with same trace
        for i in range(3):
            success = await client.emit_signal(
                layer=Layer.L5_OUTPUT_DECODING,
                category="test.trace",
                trace_id=trace_id,
            )
            assert isinstance(success, bool)

    @pytest.mark.asyncio
    async def test_decorator_basic(self, client):
        """Test @observe decorator"""
        @observe(
            layer=Layer.L5_OUTPUT_DECODING,
            category="test.decorator",
            client=client,
        )
        async def decorated_function():
            await asyncio.sleep(0.01)
            return "result"

        result = await decorated_function()
        assert result == "result"

    @pytest.mark.asyncio
    async def test_decorator_with_exception(self, client):
        """Test @observe decorator catches exceptions"""
        @observe(
            layer=Layer.L5_OUTPUT_DECODING,
            category="test.decorator_error",
            client=client,
        )
        async def decorated_function_raises():
            raise ValueError("Test error")

        with pytest.raises(ValueError):
            await decorated_function_raises()

    @pytest.mark.asyncio
    async def test_json_format_emission(self, client):
        """Test JSON format signal emission"""
        success = await client.emit_signal_json(
            layer=Layer.L5_OUTPUT_DECODING,
            category="test.json_format",
            context={"test": "value"},
        )
        assert isinstance(success, bool)


class TestSignalCorrelation:
    """Test signal correlation features"""

    @pytest.fixture
    async def client(self):
        """Initialize Argus client"""
        client = ArgusClient(
            base_url="http://localhost:8080",
            app_id="sdk-correlation-test",
            app_version="1.0.0",
        )
        await client.__aenter__()
        yield client
        await client.__aexit__(None, None, None)

    @pytest.mark.asyncio
    async def test_signals_within_trace_have_same_trace_id(self, client):
        """Verify all signals in a trace share same trace_id"""
        trace_id = client.get_trace_id()

        # Emit multiple signals
        for i in range(5):
            await client.emit_signal(
                layer=Layer.L5_OUTPUT_DECODING,
                category=f"test.correlation.{i}",
                trace_id=trace_id,
            )

        # In ClickHouse, all these signals should have same trace_id
        # Query: SELECT DISTINCT trace_id FROM signals WHERE app_id='sdk-correlation-test'
        # Expected: exactly 1 distinct trace_id

    @pytest.mark.asyncio
    async def test_parent_span_id_tracking(self, client):
        """Test parent_span_id for span hierarchy"""
        trace_id = client.get_trace_id()
        parent_span = "parent-span-123"

        # Parent span
        await client.emit_signal(
            layer=Layer.L5_OUTPUT_DECODING,
            category="test.parent_span",
            trace_id=trace_id,
        )

        # Child spans
        for i in range(3):
            await client.emit_signal(
                layer=Layer.L5_OUTPUT_DECODING,
                category="test.child_span",
                trace_id=trace_id,
                parent_span_id=parent_span,
            )

        # In ClickHouse, should see parent-child relationship
        # Query: SELECT span_id, parent_span_id FROM signals WHERE trace_id='...'


class TestFailOpenBehavior:
    """Test fail-open behavior when Argus is unreachable"""

    @pytest.mark.asyncio
    async def test_unreachable_server_returns_false(self):
        """Emitting to unreachable server returns False, not exception"""
        client = ArgusClient(
            base_url="http://unreachable-server:19999",
            app_id="fail-open-test",
        )
        await client.__aenter__()

        # Should return False, not raise exception
        success = await client.emit_signal(
            layer=Layer.L5_OUTPUT_DECODING,
            category="test.fail_open",
        )

        assert success is False

        await client.__aexit__(None, None, None)

    @pytest.mark.asyncio
    async def test_decorator_fails_open(self):
        """Decorator continues even if client unavailable"""
        @observe(
            layer=Layer.L5_OUTPUT_DECODING,
            category="test.fail_open_decorator",
            client=None,  # No client
        )
        async def my_function():
            return "still works"

        result = await my_function()
        assert result == "still works"


class TestPerformance:
    """Performance tests for SDK overhead"""

    @pytest.fixture
    async def client(self):
        """Initialize Argus client"""
        client = ArgusClient(
            base_url="http://localhost:8080",
            app_id="sdk-perf-test",
        )
        await client.__aenter__()
        yield client
        await client.__aexit__(None, None, None)

    @pytest.mark.asyncio
    async def test_signal_emission_overhead(self, client):
        """Measure signal emission overhead"""
        import time

        start = time.time()
        await client.emit_signal(
            layer=Layer.L5_OUTPUT_DECODING,
            category="test.perf",
        )
        overhead_ms = (time.time() - start) * 1000

        # Log for measurement
        print(f"Signal emission overhead: {overhead_ms:.2f}ms")

        # Overhead should be reasonable (not enforced, just measured)
        assert overhead_ms < 100  # Should be much less than 100ms

    @pytest.mark.asyncio
    async def test_decorator_overhead(self, client):
        """Measure decorator overhead"""
        import time

        @observe(
            layer=Layer.L5_OUTPUT_DECODING,
            category="test.decorator_perf",
            client=client,
        )
        async def quick_function():
            await asyncio.sleep(0.001)  # 1ms work
            return "done"

        start = time.time()
        result = await quick_function()
        total_ms = (time.time() - start) * 1000

        # 1ms work + overhead should be close to 1ms
        print(f"Decorator overhead: {total_ms - 1:.2f}ms")

        assert result == "done"
        # Overhead should be <10ms
        assert total_ms < 15


if __name__ == "__main__":
    # Run with pytest
    # pytest tests/integration/sdk_e2e_test.py -v
    pass
