"""SDK configuration."""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class AxonizeConfig:
    """Immutable SDK configuration."""

    endpoint: str
    service_name: str
    environment: str = "development"
    batch_size: int = 512
    flush_interval_ms: int = 5000
    buffer_size: int = 8192
    sampling_rate: float = 1.0
    gpu_profiling: bool = False
    gpu_snapshot_interval_ms: int = 100
    api_key: str | None = None
