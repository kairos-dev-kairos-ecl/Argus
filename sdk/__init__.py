"""
Argus SDK for Python — Signal emission and detection validation
"""

__version__ = "0.1.0"

from .client import ArgusClient, Layer, Severity, SignalContext
from .decorator import observe, ArgusObserver
from .signal_builder import SignalBuilder
from .buffer import SignalBuffer, BufferStats

__all__ = [
    "ArgusClient",
    "Layer",
    "Severity",
    "SignalContext",
    "observe",
    "ArgusObserver",
    "SignalBuilder",
    "SignalBuffer",
    "BufferStats",
]
