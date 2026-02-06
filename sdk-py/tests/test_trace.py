"""Tests for _trace decorator."""

import axonize._sdk as sdk_mod
from axonize._buffer import RingBuffer
from axonize._config import AxonizeConfig
from axonize._sdk import _AxonizeSDK
from axonize._trace import trace
from axonize._types import SpanKind


def _setup_sdk() -> RingBuffer:
    """Set up a test SDK and return its buffer."""
    cfg = AxonizeConfig(endpoint="http://test:4317", service_name="test-svc")
    sdk = _AxonizeSDK(cfg)
    sdk_mod._sdk_instance = sdk
    assert sdk._buffer is not None
    return sdk._buffer


def _teardown_sdk() -> None:
    sdk_mod._sdk_instance = None


def test_trace_creates_span() -> None:
    buf = _setup_sdk()
    try:

        @trace
        def my_func() -> str:
            return "hello"

        result = my_func()
        assert result == "hello"

        items = buf.drain(10)
        assert len(items) == 1
        assert items[0].name == "test_trace_creates_span.<locals>.my_func"
    finally:
        _teardown_sdk()


def test_trace_with_custom_name() -> None:
    buf = _setup_sdk()
    try:

        @trace(name="custom-name", kind=SpanKind.CLIENT)
        def another_func() -> int:
            return 42

        result = another_func()
        assert result == 42

        items = buf.drain(10)
        assert len(items) == 1
        assert items[0].name == "custom-name"
        assert items[0].kind == SpanKind.CLIENT
    finally:
        _teardown_sdk()


def test_trace_preserves_metadata() -> None:
    _setup_sdk()
    try:

        @trace
        def documented_func() -> None:
            """My docstring."""

        assert documented_func.__name__ == "documented_func"
        assert documented_func.__doc__ == "My docstring."
    finally:
        _teardown_sdk()


def test_trace_return_value_preserved() -> None:
    _setup_sdk()
    try:

        @trace
        def returns_dict() -> dict[str, int]:
            return {"a": 1, "b": 2}

        result = returns_dict()
        assert result == {"a": 1, "b": 2}
    finally:
        _teardown_sdk()


def test_trace_without_init_graceful() -> None:
    """@trace should work even without init (noop mode)."""
    sdk_mod._sdk_instance = None

    @trace
    def safe_func() -> str:
        return "works"

    result = safe_func()
    assert result == "works"
