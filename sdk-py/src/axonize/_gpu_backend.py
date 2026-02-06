"""GPU backend protocol and shared types for multi-vendor GPU support."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any, Protocol, runtime_checkable


@dataclass
class _GPUSnapshot:
    """Mutable metric snapshot updated by the collection thread."""

    memory_used_gb: float
    utilization: float
    temperature_celsius: int
    power_watts: int
    clock_mhz: int


@dataclass
class DiscoveredGPU:
    """A GPU device discovered by a backend at startup."""

    resource_uuid: str
    physical_gpu_uuid: str
    resource_type: str       # "full_gpu", "mig_40gb", etc.
    label: str               # "cuda:0", "mps:0"
    model: str               # "NVIDIA H100 80GB HBM3", "Apple M3 Max"
    vendor: str              # "NVIDIA", "Apple"
    node_id: str
    memory_total_gb: float
    handle: Any              # backend-specific device handle


@runtime_checkable
class GPUBackend(Protocol):
    """Structural protocol for GPU vendor backends."""

    vendor: str

    def discover(self) -> list[DiscoveredGPU]: ...

    def collect(self, handle: Any) -> _GPUSnapshot: ...

    def shutdown(self) -> None: ...
