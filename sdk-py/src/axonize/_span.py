"""Span class — the core unit of tracing."""

from __future__ import annotations

import time
import uuid
from contextvars import Token
from types import TracebackType
from typing import TYPE_CHECKING

from axonize._context import get_current_span, set_current_span
from axonize._types import GPUAttribution, SpanData, SpanKind, SpanStatus

if TYPE_CHECKING:
    from axonize._buffer import RingBuffer


class Span:
    """A mutable span that becomes an immutable SpanData on exit.

    Used as a context manager::

        with Span("my-operation", buffer=buf) as s:
            s.set_attribute("key", "value")
    """

    def __init__(
        self,
        name: str,
        *,
        buffer: RingBuffer | None,
        kind: SpanKind = SpanKind.INTERNAL,
        service_name: str = "",
        environment: str = "development",
    ) -> None:
        self.name = name
        self.kind = kind
        self._service_name = service_name
        self._environment = environment
        self._buffer = buffer

        self.span_id: str = uuid.uuid4().hex[:16]
        self._status: SpanStatus = SpanStatus.UNSET
        self._error_message: str | None = None
        self._attributes: dict[str, str | int | float | bool] = {}
        self._gpu_labels: list[str] = []
        self._gpu_attributions: list[GPUAttribution] = []

        # Parent/trace resolution
        parent = get_current_span()
        if parent is not None:
            self.trace_id: str = parent.trace_id
            self.parent_span_id: str | None = parent.span_id
        else:
            self.trace_id = uuid.uuid4().hex
            self.parent_span_id = None

        self._start_time_ns: int = 0
        self._end_time_ns: int = 0
        self._token: Token[Span | None] | None = None

    def __enter__(self) -> Span:
        self._start_time_ns = time.time_ns()
        self._token = set_current_span(self)
        return self

    def __exit__(
        self,
        exc_type: type[BaseException] | None,
        exc_val: BaseException | None,
        exc_tb: TracebackType | None,
    ) -> None:
        self._end_time_ns = time.time_ns()

        if exc_type is not None:
            self._status = SpanStatus.ERROR
            self._error_message = str(exc_val) if exc_val else exc_type.__name__
        elif self._status == SpanStatus.UNSET:
            self._status = SpanStatus.OK

        # Restore parent context
        if self._token is not None:
            from axonize._context import _current_span

            _current_span.reset(self._token)

        # Enqueue immutable snapshot
        if self._buffer is not None:
            self._buffer.enqueue(self._to_span_data())

    def set_attribute(self, key: str, value: str | int | float | bool) -> None:
        """Attach a key-value attribute to this span."""
        self._attributes[key] = value

    def set_gpus(self, labels: list[str]) -> None:
        """Set GPU device labels (e.g. ["cuda:0", "cuda:1"]).

        If a GPU profiler is active, automatically resolves labels to full
        GPUAttribution objects with hardware identity and live metrics.
        """
        self._gpu_labels = list(labels)
        from axonize._sdk import _get_sdk

        sdk = _get_sdk()
        if hasattr(sdk, "_gpu_profiler") and sdk._gpu_profiler is not None:
            self._gpu_attributions = sdk._gpu_profiler.resolve_labels(labels)
        else:
            self._gpu_attributions = []

    def set_status(self, status: SpanStatus, message: str | None = None) -> None:
        """Explicitly set span status."""
        self._status = status
        self._error_message = message

    def _to_span_data(self) -> SpanData:
        duration_ms = (self._end_time_ns - self._start_time_ns) / 1_000_000
        return SpanData(
            span_id=self.span_id,
            trace_id=self.trace_id,
            name=self.name,
            kind=self.kind,
            status=self._status,
            start_time_ns=self._start_time_ns,
            end_time_ns=self._end_time_ns,
            duration_ms=duration_ms,
            service_name=self._service_name,
            attributes=dict(self._attributes),
            parent_span_id=self.parent_span_id,
            gpu_attributions=list(self._gpu_attributions),
            error_message=self._error_message,
            environment=self._environment,
        )
