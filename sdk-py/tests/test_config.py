"""Tests for _config module."""

from axonize._config import AxonizeConfig


def test_config_defaults() -> None:
    cfg = AxonizeConfig(endpoint="http://localhost:4317", service_name="test")
    assert cfg.endpoint == "http://localhost:4317"
    assert cfg.service_name == "test"
    assert cfg.environment == "development"
    assert cfg.batch_size == 512
    assert cfg.flush_interval_ms == 5000
    assert cfg.buffer_size == 8192
    assert cfg.sampling_rate == 1.0
    assert cfg.gpu_profiling is False


def test_config_custom_values() -> None:
    cfg = AxonizeConfig(
        endpoint="http://prod:4317",
        service_name="prod-svc",
        environment="production",
        batch_size=1024,
        flush_interval_ms=1000,
        buffer_size=16384,
        sampling_rate=0.5,
        gpu_profiling=True,
    )
    assert cfg.environment == "production"
    assert cfg.batch_size == 1024
    assert cfg.flush_interval_ms == 1000
    assert cfg.buffer_size == 16384
    assert cfg.sampling_rate == 0.5
    assert cfg.gpu_profiling is True


def test_config_is_frozen() -> None:
    cfg = AxonizeConfig(endpoint="http://localhost:4317", service_name="test")
    try:
        cfg.endpoint = "changed"  # type: ignore[misc]
        assert False, "Should have raised"
    except AttributeError:
        pass
