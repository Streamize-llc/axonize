"""Tests for _buffer module."""

import threading

from axonize._buffer import RingBuffer
from axonize._types import SpanData, SpanKind, SpanStatus


def _make_span(name: str = "test") -> SpanData:
    return SpanData(
        span_id="s1",
        trace_id="t1",
        name=name,
        kind=SpanKind.INTERNAL,
        status=SpanStatus.OK,
        start_time_ns=0,
        end_time_ns=1000,
        duration_ms=0.001,
        service_name="svc",
    )


def test_enqueue_and_drain() -> None:
    buf = RingBuffer(maxsize=10)
    buf.enqueue(_make_span("a"))
    buf.enqueue(_make_span("b"))
    assert len(buf) == 2

    items = buf.drain(10)
    assert len(items) == 2
    assert items[0].name == "a"
    assert items[1].name == "b"
    assert len(buf) == 0


def test_drain_partial() -> None:
    buf = RingBuffer(maxsize=10)
    for i in range(5):
        buf.enqueue(_make_span(f"s{i}"))

    items = buf.drain(3)
    assert len(items) == 3
    assert len(buf) == 2


def test_drain_empty() -> None:
    buf = RingBuffer(maxsize=10)
    items = buf.drain(10)
    assert items == []


def test_overflow_drops_oldest() -> None:
    buf = RingBuffer(maxsize=3)
    buf.enqueue(_make_span("a"))
    buf.enqueue(_make_span("b"))
    buf.enqueue(_make_span("c"))
    assert buf.drop_count == 0

    buf.enqueue(_make_span("d"))
    assert buf.drop_count == 1
    assert len(buf) == 3

    items = buf.drain(10)
    assert [s.name for s in items] == ["b", "c", "d"]


def test_drop_count_accumulates() -> None:
    buf = RingBuffer(maxsize=2)
    for i in range(10):
        buf.enqueue(_make_span(f"s{i}"))
    assert buf.drop_count == 8
    assert len(buf) == 2


def test_concurrent_enqueue() -> None:
    buf = RingBuffer(maxsize=10000)
    n_threads = 4
    n_per_thread = 500

    def writer() -> None:
        for i in range(n_per_thread):
            buf.enqueue(_make_span(f"s{i}"))

    threads = [threading.Thread(target=writer) for _ in range(n_threads)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()

    total = len(buf.drain(n_threads * n_per_thread + 1))
    assert total == n_threads * n_per_thread
