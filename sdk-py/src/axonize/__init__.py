"""Axonize: Observability SDK for AI inference workloads."""

from __future__ import annotations

from typing import TYPE_CHECKING

from axonize._sdk import _get_sdk, init, shutdown
from axonize._span import Span
from axonize._trace import trace
from axonize._types import GPUAttribution, SpanData, SpanKind, SpanStatus

if TYPE_CHECKING:
    pass

__version__ = "0.1.0"

__all__ = [
    "GPUAttribution",
    "Span",
    "SpanData",
    "SpanKind",
    "SpanStatus",
    "__version__",
    "init",
    "shutdown",
    "span",
    "trace",
]


def span(
    name: str,
    *,
    kind: SpanKind = SpanKind.INTERNAL,
) -> Span:
    """Create a span context manager.

    Usage::

        with axonize.span("process-batch") as s:
            s.set_attribute("batch_size", 32)
    """
    sdk = _get_sdk()
    return sdk.create_span(name, kind=kind)
