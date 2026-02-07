"""GPU profiler — multi-vendor backend with 3-Layer Identity and async metric collection."""

from __future__ import annotations

import logging
import sys
import threading
from dataclasses import dataclass

from axonize._gpu_backend import GPUBackend, _GPUSnapshot
from axonize._types import GPUAttribution

logger = logging.getLogger("axonize.gpu")


@dataclass
class _GPUStaticInfo:
    """Immutable hardware info discovered at startup."""

    model: str
    vendor: str
    node_id: str
    resource_type: str
    physical_gpu_uuid: str
    memory_total_gb: float


class _GPUResolverMixin:
    """Shared resolve_labels() logic for GPUProfiler and MockGPUProfiler."""

    _label_to_resource: dict[str, str]
    _snapshots: dict[str, _GPUSnapshot]
    _gpu_info: dict[str, _GPUStaticInfo]

    def resolve_labels(self, labels: list[str]) -> list[GPUAttribution]:
        result: list[GPUAttribution] = []
        for label in labels:
            resource_uuid = self._label_to_resource.get(label)
            if resource_uuid is None:
                continue
            snapshot = self._snapshots.get(resource_uuid)
            info = self._gpu_info.get(resource_uuid)
            if snapshot is None or info is None:
                continue
            result.append(GPUAttribution(
                resource_uuid=resource_uuid,
                physical_gpu_uuid=info.physical_gpu_uuid,
                gpu_model=info.model,
                vendor=info.vendor,
                node_id=info.node_id,
                resource_type=info.resource_type,
                user_label=label,
                memory_used_gb=snapshot.memory_used_gb,
                memory_total_gb=info.memory_total_gb,
                utilization=snapshot.utilization,
                temperature_celsius=snapshot.temperature_celsius,
                power_watts=snapshot.power_watts,
                clock_mhz=snapshot.clock_mhz,
            ))
        return result


class GPUProfiler(_GPUResolverMixin):
    """Real GPU profiler using a pluggable backend.

    Discovers GPUs at init, collects metrics in a daemon thread.
    """

    def __init__(self, *, backend: GPUBackend, snapshot_interval_ms: int = 100) -> None:
        self._backend = backend
        self._interval_s = snapshot_interval_ms / 1000.0
        self._stop_event = threading.Event()
        self._thread: threading.Thread | None = None

        self._label_to_resource: dict[str, str] = {}
        self._resource_to_physical: dict[str, str] = {}
        self._snapshots: dict[str, _GPUSnapshot] = {}
        self._gpu_info: dict[str, _GPUStaticInfo] = {}
        self._handles: dict[str, object] = {}

        self._discover_gpus()

    def _discover_gpus(self) -> None:
        for gpu in self._backend.discover():
            self._label_to_resource[gpu.label] = gpu.resource_uuid
            self._resource_to_physical[gpu.resource_uuid] = gpu.physical_gpu_uuid
            self._gpu_info[gpu.resource_uuid] = _GPUStaticInfo(
                model=gpu.model,
                vendor=gpu.vendor,
                node_id=gpu.node_id,
                resource_type=gpu.resource_type,
                physical_gpu_uuid=gpu.physical_gpu_uuid,
                memory_total_gb=gpu.memory_total_gb,
            )
            self._handles[gpu.resource_uuid] = gpu.handle
            self._snapshots[gpu.resource_uuid] = _GPUSnapshot(
                memory_used_gb=0.0,
                utilization=0.0,
                temperature_celsius=0,
                power_watts=0,
                clock_mhz=0,
            )

    def start(self) -> None:
        if self._thread is not None:
            return
        self._stop_event.clear()
        self._thread = threading.Thread(target=self._collection_loop, daemon=True)
        self._thread.start()

    def stop(self) -> None:
        self._stop_event.set()
        if self._thread is not None:
            self._thread.join(timeout=2.0)
            self._thread = None
        self._backend.shutdown()

    def _collection_loop(self) -> None:
        while not self._stop_event.wait(self._interval_s):
            for resource_uuid, handle in self._handles.items():
                try:
                    # CPython GIL guarantees dict.__setitem__ is atomic for
                    # reader threads calling resolve_labels() concurrently.
                    self._snapshots[resource_uuid] = self._backend.collect(handle)
                except Exception as exc:  # noqa: BLE001
                    logger.debug(
                        "GPU collect failed for %s: %s", resource_uuid, exc, exc_info=True,
                    )


class MockGPUProfiler(_GPUResolverMixin):
    """Test-only profiler that simulates GPU discovery and metrics without a real backend."""

    def __init__(
        self,
        *,
        num_gpus: int = 2,
        mig_enabled: bool = False,
        vendor: str = "NVIDIA",
    ) -> None:
        self._label_to_resource: dict[str, str] = {}
        self._resource_to_physical: dict[str, str] = {}
        self._snapshots: dict[str, _GPUSnapshot] = {}
        self._gpu_info: dict[str, _GPUStaticInfo] = {}

        node_id = "test-node"
        cuda_idx = 0
        label_prefix = "cuda" if vendor == "NVIDIA" else "mps"

        for i in range(num_gpus):
            gpu_uuid = f"GPU-{i:04d}"

            if mig_enabled:
                for j in range(2):
                    mig_uuid = f"MIG-{i:04d}-{j:02d}"
                    label = f"{label_prefix}:{cuda_idx}"
                    self._label_to_resource[label] = mig_uuid
                    self._resource_to_physical[mig_uuid] = gpu_uuid
                    self._gpu_info[mig_uuid] = _GPUStaticInfo(
                        model="NVIDIA H100 80GB HBM3",
                        vendor=vendor,
                        node_id=node_id,
                        resource_type="mig_40gb",
                        physical_gpu_uuid=gpu_uuid,
                        memory_total_gb=40.0,
                    )
                    self._snapshots[mig_uuid] = _GPUSnapshot(
                        memory_used_gb=20.0 + j,
                        utilization=75.0 + j * 5,
                        temperature_celsius=65 + j,
                        power_watts=200 + j * 10,
                        clock_mhz=1500,
                    )
                    cuda_idx += 1
            else:
                label = f"{label_prefix}:{cuda_idx}"
                self._label_to_resource[label] = gpu_uuid
                self._resource_to_physical[gpu_uuid] = gpu_uuid
                self._gpu_info[gpu_uuid] = _GPUStaticInfo(
                    model="NVIDIA H100 80GB HBM3" if vendor == "NVIDIA" else "Apple M3 Max",
                    vendor=vendor,
                    node_id=node_id,
                    resource_type="full_gpu",
                    physical_gpu_uuid=gpu_uuid,
                    memory_total_gb=80.0 if vendor == "NVIDIA" else 36.0,
                )
                self._snapshots[gpu_uuid] = _GPUSnapshot(
                    memory_used_gb=42.0 + i,
                    utilization=85.0 + i,
                    temperature_celsius=72 + i,
                    power_watts=350 + i * 10,
                    clock_mhz=1800,
                )
                cuda_idx += 1

    def start(self) -> None:
        pass

    def stop(self) -> None:
        pass


def create_gpu_profiler(
    *, snapshot_interval_ms: int = 100
) -> GPUProfiler | MockGPUProfiler | None:
    """Factory: returns a GPUProfiler with the best available backend, else None."""
    # 1) Try NVIDIA
    try:
        from axonize._gpu_nvml import NvmlBackend

        return GPUProfiler(backend=NvmlBackend(), snapshot_interval_ms=snapshot_interval_ms)
    except Exception:  # noqa: BLE001
        pass

    # 2) Try Apple Silicon (macOS ARM64)
    if sys.platform == "darwin":
        try:
            from axonize._gpu_apple import AppleSiliconBackend

            return GPUProfiler(
                backend=AppleSiliconBackend(), snapshot_interval_ms=snapshot_interval_ms
            )
        except Exception:  # noqa: BLE001
            pass

    logger.info("No GPU backend available — GPU profiling disabled")
    return None
