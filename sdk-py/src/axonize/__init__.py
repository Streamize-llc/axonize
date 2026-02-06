"""Axonize: Observability SDK for AI inference workloads."""

from __future__ import annotations

from typing import TYPE_CHECKING

from axonize._llm import LLMSpan
from axonize._sdk import _get_sdk, init, shutdown
from axonize._span import Span
from axonize._trace import trace
from axonize._types import GPUAttribution, SpanData, SpanKind, SpanStatus

if TYPE_CHECKING:
    pass

__version__ = "0.1.0"

__all__ = [
    "GPUAttribution",
    "LLMSpan",
    "Span",
    "SpanData",
    "SpanKind",
    "SpanStatus",
    "__version__",
    "init",
    "llm_span",
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


def llm_span(
    name: str,
    *,
    model: str | None = None,
    model_version: str | None = None,
    inference_type: str = "llm",
    kind: SpanKind = SpanKind.SERVER,
) -> LLMSpan:
    """Create an LLM-specialized span with token tracking and TTFT/TPOT.

    Usage::

        with axonize.llm_span("generate", model="llama-3-70b") as s:
            s.set_tokens_input(128)
            for token in model.generate_stream(prompt):
                s.record_token()
                yield token
    """
    sdk = _get_sdk()
    return sdk.create_llm_span(
        name,
        model=model,
        model_version=model_version,
        inference_type=inference_type,
        kind=kind,
    )
