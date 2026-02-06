"""Tests for _types module."""

from axonize._types import SpanData, SpanKind, SpanStatus


def test_span_kind_values() -> None:
    assert SpanKind.INTERNAL.value == "internal"
    assert SpanKind.CLIENT.value == "client"
    assert SpanKind.SERVER.value == "server"


def test_span_status_values() -> None:
    assert SpanStatus.UNSET.value == "unset"
    assert SpanStatus.OK.value == "ok"
    assert SpanStatus.ERROR.value == "error"


def test_span_data_creation() -> None:
    sd = SpanData(
        span_id="abc123",
        trace_id="trace456",
        name="test-span",
        kind=SpanKind.INTERNAL,
        status=SpanStatus.OK,
        start_time_ns=1000,
        end_time_ns=2000,
        duration_ms=0.001,
        service_name="test-svc",
    )
    assert sd.span_id == "abc123"
    assert sd.trace_id == "trace456"
    assert sd.name == "test-span"
    assert sd.kind == SpanKind.INTERNAL
    assert sd.status == SpanStatus.OK
    assert sd.parent_span_id is None
    assert sd.attributes == {}
    assert sd.gpu_attributions == []
    assert sd.error_message is None
    assert sd.environment == "development"


def test_span_data_is_frozen() -> None:
    sd = SpanData(
        span_id="a",
        trace_id="b",
        name="c",
        kind=SpanKind.INTERNAL,
        status=SpanStatus.OK,
        start_time_ns=0,
        end_time_ns=0,
        duration_ms=0.0,
        service_name="svc",
    )
    try:
        sd.span_id = "changed"  # type: ignore[misc]
        assert False, "Should have raised"
    except AttributeError:
        pass


def test_span_data_with_optional_fields() -> None:
    sd = SpanData(
        span_id="a",
        trace_id="b",
        name="c",
        kind=SpanKind.SERVER,
        status=SpanStatus.ERROR,
        start_time_ns=100,
        end_time_ns=200,
        duration_ms=0.0001,
        service_name="svc",
        parent_span_id="parent1",
        attributes={"key": "val"},
        gpu_attributions=[],
        error_message="boom",
        environment="production",
    )
    assert sd.parent_span_id == "parent1"
    assert sd.attributes == {"key": "val"}
    assert sd.gpu_attributions == []
    assert sd.error_message == "boom"
    assert sd.environment == "production"
