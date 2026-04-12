"""
OpenTelemetry integration for Argus SDK

Converts OTLP spans to Argus signals via gRPC receiver.
"""

__version__ = "0.1.0"

from .receiver import OTLPReceiver
from .converter import OTLPToArgusConverter

__all__ = ["OTLPReceiver", "OTLPToArgusConverter"]
