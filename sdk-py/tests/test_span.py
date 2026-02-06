"""Tests for _span module."""

import time

from axonize._buffer import RingBuffer
from axonize._context import get_current_span
from axonize._span import Span
from axonize._types import SpanKind, SpanStatus


def test_span_timing() -> None:
    buf = RingBuffer(maxsize=10)
    with Span("timed", buffer=buf):
        time.sleep(0.01)

    items = buf.drain(1)
    assert len(items) == 1
    sd = items[0]
    assert sd.start_time_ns > 0
    assert sd.end_time_ns > sd.start_time_ns
    assert sd.duration_ms >= 10.0  # at least 10ms


def test_span_auto_ok() -> None:
    buf = RingBuffer(maxsize=10)
    with Span("ok-span", buffer=buf):
        pass

    items = buf.drain(1)
    assert items[0].status == SpanStatus.OK


def test_span_auto_error_on_exception() -> None:
    buf = RingBuffer(maxsize=10)
    try:
        with Span("err-span", buffer=buf):
            raise ValueError("test error")
    except ValueError:
        pass

    items = buf.drain(1)
    sd = items[0]
    assert sd.status == SpanStatus.ERROR
    assert sd.error_message == "test error"


def test_span_exception_propagates() -> None:
    buf = RingBuffer(maxsize=10)
    caught = False
    try:
        with Span("err-span", buffer=buf):
            raise RuntimeError("propagate me")
    except RuntimeError:
        caught = True

    assert caught


def test_span_set_attribute() -> None:
    buf = RingBuffer(maxsize=10)
    with Span("attr-span", buffer=buf) as s:
        s.set_attribute("model", "llama-3")
        s.set_attribute("batch_size", 32)
        s.set_attribute("temperature", 0.7)
        s.set_attribute("stream", True)

    items = buf.drain(1)
    attrs = items[0].attributes
    assert attrs["model"] == "llama-3"
    assert attrs["batch_size"] == 32
    assert attrs["temperature"] == 0.7
    assert attrs["stream"] is True


def test_span_set_gpus() -> None:
    buf = RingBuffer(maxsize=10)
    with Span("gpu-span", buffer=buf) as s:
        s.set_gpus(["cuda:0", "cuda:1"])

    items = buf.drain(1)
    # Without a profiler, gpu_attributions is empty but labels are stored internally
    assert items[0].gpu_attributions == []


def test_span_set_status() -> None:
    buf = RingBuffer(maxsize=10)
    with Span("status-span", buffer=buf) as s:
        s.set_status(SpanStatus.ERROR, "manual error")

    items = buf.drain(1)
    sd = items[0]
    assert sd.status == SpanStatus.ERROR
    assert sd.error_message == "manual error"


def test_span_parent_child() -> None:
    buf = RingBuffer(maxsize=10)
    with Span("parent", buffer=buf) as parent:
        with Span("child", buffer=buf) as child:
            assert child.trace_id == parent.trace_id
            assert child.parent_span_id == parent.span_id

    items = buf.drain(10)
    assert len(items) == 2
    child_data = items[0]
    parent_data = items[1]
    assert child_data.parent_span_id == parent_data.span_id
    assert child_data.trace_id == parent_data.trace_id


def test_root_span_has_no_parent() -> None:
    buf = RingBuffer(maxsize=10)
    with Span("root", buffer=buf):
        pass

    items = buf.drain(1)
    assert items[0].parent_span_id is None


def test_span_ids_are_correct_length() -> None:
    buf = RingBuffer(maxsize=10)
    with Span("id-test", buffer=buf) as s:
        assert len(s.span_id) == 16
        assert len(s.trace_id) == 32


def test_span_enqueues_to_buffer() -> None:
    buf = RingBuffer(maxsize=10)
    with Span("buffered", buffer=buf):
        pass
    assert len(buf) == 1


def test_span_with_none_buffer() -> None:
    """Span with no buffer should work (graceful degradation)."""
    with Span("no-buffer", buffer=None) as s:
        s.set_attribute("key", "val")
    # No error raised


def test_context_restored_after_exit() -> None:
    assert get_current_span() is None
    buf = RingBuffer(maxsize=10)
    with Span("outer", buffer=buf):
        with Span("inner", buffer=buf):
            pass
        # After inner exits, current span should be outer
    # After outer exits, current span should be None
    assert get_current_span() is None


def test_span_service_name_and_environment() -> None:
    buf = RingBuffer(maxsize=10)
    with Span(
        "svc-test",
        buffer=buf,
        service_name="my-svc",
        environment="production",
    ):
        pass

    items = buf.drain(1)
    assert items[0].service_name == "my-svc"
    assert items[0].environment == "production"


def test_span_kind() -> None:
    buf = RingBuffer(maxsize=10)
    with Span("client-span", buffer=buf, kind=SpanKind.CLIENT):
        pass

    items = buf.drain(1)
    assert items[0].kind == SpanKind.CLIENT
