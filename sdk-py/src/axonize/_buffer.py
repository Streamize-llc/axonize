"""Lock-free ring buffer for span data."""

from __future__ import annotations

from collections import deque

from axonize._types import SpanData


class RingBuffer:
    """Thread-safe ring buffer backed by collections.deque.

    CPython's GIL guarantees that deque.append and deque.popleft are atomic,
    so no explicit locking is needed for single-producer/single-consumer usage.
    """

    def __init__(self, maxsize: int) -> None:
        self._buffer: deque[SpanData] = deque(maxlen=maxsize)
        self._drop_count: int = 0
        self._maxsize = maxsize

    def enqueue(self, span: SpanData) -> None:
        """Add a span to the buffer. Oldest item is dropped if full."""
        if len(self._buffer) == self._maxsize:
            self._drop_count += 1
        self._buffer.append(span)

    def drain(self, max_items: int) -> list[SpanData]:
        """Remove and return up to max_items spans from the buffer."""
        items: list[SpanData] = []
        for _ in range(max_items):
            try:
                items.append(self._buffer.popleft())
            except IndexError:
                break
        return items

    @property
    def drop_count(self) -> int:
        """Number of spans dropped due to buffer overflow."""
        return self._drop_count

    def __len__(self) -> int:
        return len(self._buffer)
