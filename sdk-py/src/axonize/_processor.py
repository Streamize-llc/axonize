"""Background processor that drains the ring buffer periodically."""

from __future__ import annotations

import threading
from collections.abc import Callable

from axonize._buffer import RingBuffer
from axonize._types import SpanData

SpanHandler = Callable[[list[SpanData]], None]


def _noop_handler(spans: list[SpanData]) -> None:
    """Default handler that discards spans. Replaced by OTLP exporter in M2."""


class BackgroundProcessor:
    """Daemon thread that periodically drains spans from the buffer."""

    def __init__(
        self,
        buffer: RingBuffer,
        *,
        batch_size: int = 512,
        flush_interval_ms: int = 5000,
        handler: SpanHandler = _noop_handler,
    ) -> None:
        self._buffer = buffer
        self._batch_size = batch_size
        self._flush_interval_s = flush_interval_ms / 1000.0
        self._handler = handler
        self._stop_event = threading.Event()
        self._thread: threading.Thread | None = None

    def start(self) -> None:
        """Start the background drain loop."""
        if self._thread is not None:
            return
        self._stop_event.clear()
        self._thread = threading.Thread(target=self._run, daemon=True)
        self._thread.start()

    def stop(self) -> None:
        """Signal stop and perform a final drain."""
        self._stop_event.set()
        if self._thread is not None:
            self._thread.join(timeout=5.0)
            self._thread = None
        self._flush()

    def _run(self) -> None:
        while not self._stop_event.wait(timeout=self._flush_interval_s):
            self._flush()

    def _flush(self) -> None:
        spans = self._buffer.drain(self._batch_size)
        if spans:
            try:
                self._handler(spans)
            except Exception:  # noqa: BLE001
                pass  # Graceful degradation — never crash the drain loop

    @property
    def is_running(self) -> bool:
        return self._thread is not None and self._thread.is_alive()
