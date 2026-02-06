"""Tests for _processor module."""

import time

from axonize._buffer import RingBuffer
from axonize._processor import BackgroundProcessor
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


def test_start_and_stop() -> None:
    buf = RingBuffer(maxsize=100)
    proc = BackgroundProcessor(buf, flush_interval_ms=50)
    proc.start()
    assert proc.is_running
    proc.stop()
    assert not proc.is_running


def test_handler_called_on_flush() -> None:
    buf = RingBuffer(maxsize=100)
    received: list[SpanData] = []

    def handler(spans: list[SpanData]) -> None:
        received.extend(spans)

    proc = BackgroundProcessor(buf, flush_interval_ms=50, handler=handler)

    buf.enqueue(_make_span("a"))
    buf.enqueue(_make_span("b"))

    proc.start()
    time.sleep(0.2)
    proc.stop()

    assert len(received) >= 2
    names = [s.name for s in received]
    assert "a" in names
    assert "b" in names


def test_final_drain_on_stop() -> None:
    buf = RingBuffer(maxsize=100)
    received: list[SpanData] = []

    def handler(spans: list[SpanData]) -> None:
        received.extend(spans)

    proc = BackgroundProcessor(
        buf, flush_interval_ms=10000, handler=handler  # Long interval
    )
    proc.start()

    buf.enqueue(_make_span("late"))
    proc.stop()

    names = [s.name for s in received]
    assert "late" in names


def test_handler_exception_does_not_crash() -> None:
    buf = RingBuffer(maxsize=100)

    def bad_handler(spans: list[SpanData]) -> None:
        raise RuntimeError("handler exploded")

    proc = BackgroundProcessor(buf, flush_interval_ms=50, handler=bad_handler)
    buf.enqueue(_make_span())
    proc.start()
    time.sleep(0.15)
    proc.stop()
    # Should not raise


def test_thread_is_daemon() -> None:
    buf = RingBuffer(maxsize=100)
    proc = BackgroundProcessor(buf, flush_interval_ms=50)
    proc.start()
    assert proc._thread is not None
    assert proc._thread.daemon is True
    proc.stop()


def test_double_start_is_idempotent() -> None:
    buf = RingBuffer(maxsize=100)
    proc = BackgroundProcessor(buf, flush_interval_ms=50)
    proc.start()
    thread1 = proc._thread
    proc.start()  # Should not create a second thread
    assert proc._thread is thread1
    proc.stop()
