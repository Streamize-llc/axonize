"""Tests for _sdk module."""

import axonize
import axonize._sdk as sdk_mod


def setup_function() -> None:
    """Reset SDK state before each test."""
    sdk_mod._sdk_instance = None


def test_init_creates_sdk() -> None:
    axonize.init(endpoint="http://localhost:4317", service_name="test")
    assert sdk_mod._sdk_instance is not None
    assert sdk_mod._sdk_instance.config.service_name == "test"
    axonize.shutdown()


def test_shutdown_clears_sdk() -> None:
    axonize.init(endpoint="http://localhost:4317", service_name="test")
    axonize.shutdown()
    assert sdk_mod._sdk_instance is None


def test_reinit_shuts_down_previous() -> None:
    axonize.init(endpoint="http://localhost:4317", service_name="first")
    first = sdk_mod._sdk_instance
    axonize.init(endpoint="http://localhost:4317", service_name="second")
    assert sdk_mod._sdk_instance is not first
    assert sdk_mod._sdk_instance is not None
    assert sdk_mod._sdk_instance.config.service_name == "second"
    axonize.shutdown()


def test_uninit_graceful_span() -> None:
    """span() should work without init — no error, just silently discarded."""
    with axonize.span("graceful") as s:
        s.set_attribute("key", "val")
    # No error raised


def test_uninit_graceful_trace() -> None:
    """@trace should work without init."""

    @axonize.trace
    def my_func() -> str:
        return "ok"

    assert my_func() == "ok"


def test_init_with_custom_config() -> None:
    axonize.init(
        endpoint="http://prod:4317",
        service_name="prod-svc",
        environment="production",
        batch_size=1024,
        buffer_size=16384,
        flush_interval_ms=1000,
        sampling_rate=0.5,
        gpu_profiling=True,
    )
    assert sdk_mod._sdk_instance is not None
    cfg = sdk_mod._sdk_instance.config
    assert cfg.environment == "production"
    assert cfg.batch_size == 1024
    assert cfg.buffer_size == 16384
    axonize.shutdown()


def test_span_after_init_enqueues() -> None:
    axonize.init(endpoint="http://localhost:4317", service_name="test")
    assert sdk_mod._sdk_instance is not None

    with axonize.span("buffered") as s:
        s.set_attribute("key", "val")

    buf = sdk_mod._sdk_instance._buffer
    assert buf is not None
    assert len(buf) == 1
    axonize.shutdown()
