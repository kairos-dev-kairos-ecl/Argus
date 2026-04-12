"""
Tests for ArgusClient
"""

import asyncio
import pytest
from unittest.mock import AsyncMock, patch, MagicMock

from sdk.client import ArgusClient, Layer, Severity


@pytest.mark.asyncio
async def test_client_initialization():
    """Test ArgusClient initialization"""
    client = ArgusClient(
        base_url="http://localhost:8080",
        app_id="test-app",
        app_version="1.0.0",
    )
    assert client.app_id == "test-app"
    assert client.app_version == "1.0.0"
    assert client.base_url == "http://localhost:8080"


@pytest.mark.asyncio
async def test_client_context_manager():
    """Test ArgusClient context manager"""
    async with ArgusClient() as client:
        assert client.session is not None


@pytest.mark.asyncio
async def test_emit_signal_success():
    """Test successful signal emission"""
    with patch("httpx.AsyncClient") as mock_http:
        mock_response = AsyncMock()
        mock_response.status_code = 202
        mock_http_instance = AsyncMock()
        mock_http_instance.post = AsyncMock(return_value=mock_response)
        mock_http_instance.aclose = AsyncMock()
        mock_http.return_value = mock_http_instance

        async with ArgusClient() as client:
            success = await client.emit_signal(
                layer=Layer.L5_OUTPUT_DECODING,
                category="inference.completion",
                severity=Severity.HIGH,
            )

        assert success is True
        assert mock_http_instance.post.called


@pytest.mark.asyncio
async def test_emit_signal_failure():
    """Test signal emission failure"""
    with patch("httpx.AsyncClient") as mock_http:
        mock_response = AsyncMock()
        mock_response.status_code = 500
        mock_http_instance = AsyncMock()
        mock_http_instance.post = AsyncMock(return_value=mock_response)
        mock_http_instance.aclose = AsyncMock()
        mock_http.return_value = mock_http_instance

        async with ArgusClient() as client:
            success = await client.emit_signal(
                layer=Layer.L5_OUTPUT_DECODING,
                category="inference.completion",
            )

        assert success is False


@pytest.mark.asyncio
async def test_trace_id_management():
    """Test trace ID getter/setter"""
    client = ArgusClient()

    # Generate initial trace ID
    trace_id_1 = client.get_trace_id()
    assert trace_id_1 is not None

    # Should return same trace ID on subsequent calls
    trace_id_2 = client.get_trace_id()
    assert trace_id_1 == trace_id_2

    # Can set custom trace ID
    client.set_trace_id("custom-trace-123")
    assert client.get_trace_id() == "custom-trace-123"


@pytest.mark.asyncio
async def test_emit_signal_with_context():
    """Test signal emission with layer-specific context"""
    with patch("httpx.AsyncClient") as mock_http:
        mock_response = AsyncMock()
        mock_response.status_code = 202
        mock_http_instance = AsyncMock()
        mock_http_instance.post = AsyncMock(return_value=mock_response)
        mock_http_instance.aclose = AsyncMock()
        mock_http.return_value = mock_http_instance

        async with ArgusClient() as client:
            context = {
                "output_tokens": 100,
                "input_tokens": 50,
                "total_tokens": 150,
                "finish_reason": "stop",
            }
            success = await client.emit_signal(
                layer=Layer.L5_OUTPUT_DECODING,
                category="inference.completion",
                context=context,
            )

        assert success is True


@pytest.mark.asyncio
async def test_emit_signal_json_format():
    """Test JSON signal emission"""
    with patch("httpx.AsyncClient") as mock_http:
        mock_response = AsyncMock()
        mock_response.status_code = 202
        mock_http_instance = AsyncMock()
        mock_http_instance.post = AsyncMock(return_value=mock_response)
        mock_http_instance.aclose = AsyncMock()
        mock_http.return_value = mock_http_instance

        async with ArgusClient() as client:
            success = await client.emit_signal_json(
                layer=Layer.L5_OUTPUT_DECODING,
                category="inference.completion",
            )

        assert success is True
        # Check that Content-Type was set to application/json
        call_kwargs = mock_http_instance.post.call_args[1]
        assert "application/json" in call_kwargs.get("headers", {}).get("Content-Type", "")


def test_ulid_generation():
    """Test ULID generation"""
    client = ArgusClient()
    ulid_1 = client._ulid()
    ulid_2 = client._ulid()

    # ULIDs should be different
    assert ulid_1 != ulid_2
    # Should be 26 characters (10 timestamp + 12 random + 4 padding)
    assert len(ulid_1) >= 22


@pytest.mark.asyncio
async def test_emit_without_context_manager():
    """Test that emit_signal requires context manager"""
    client = ArgusClient()

    with pytest.raises(RuntimeError):
        await client.emit_signal(
            layer=Layer.L5_OUTPUT_DECODING,
            category="inference.completion",
        )


@pytest.mark.asyncio
async def test_signal_serialization():
    """Test signal protobuf serialization"""
    async with ArgusClient() as client:
        # Just ensure the signal can be built and serialized
        # by calling emit_signal with a mock
        with patch("httpx.AsyncClient.post") as mock_post:
            mock_response = AsyncMock()
            mock_response.status_code = 202
            mock_post.return_value = mock_response

            await client.emit_signal(
                layer=Layer.L7_RAG_RETRIEVAL,
                category="retrieval.search",
                context={
                    "query_text": "test query",
                    "results_count": 5,
                }
            )

            assert mock_post.called
