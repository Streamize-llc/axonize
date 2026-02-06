"""GPU profiler — pynvml wrapper with 3-Layer Identity and async metric collection."""

from __future__ import annotations

import logging
import platform
import threading
import warnings
from dataclasses import dataclass

from axonize._types import GPUAttribution

logger = logging.getLogger("axonize.gpu")

# pynvml is optional — profiler degrades gracefully when unavailable.
# Suppress deprecation warning from pynvml (recommends nvidia-ml-py).
warnings.filterwarnings("ignore", category=FutureWarning, message=".*pynvml.*deprecated.*")
try:
    import pynvml

    _HAS_PYNVML = True
except ImportError:
    pynvml = None  # type: ignore[assignment,unused-ignore]
    _HAS_PYNVML = False


@dataclass
class _GPUStaticInfo:
    """Immutable hardware info discovered at startup."""

    model: str
    node_id: str
    resource_type: str
    physical_gpu_uuid: str
    memory_total_gb: float


@dataclass
class _GPUSnapshot:
    """Mutable metric snapshot updated by the collection thread."""

    memory_used_gb: float
    utilization: float
    temperature_celsius: int
    power_watts: int
    clock_mhz: int


class GPUProfiler:
    """Real GPU profiler using pynvml.

    Discovers GPUs at init, collects metrics in a daemon thread.
    """

    def __init__(self, *, snapshot_interval_ms: int = 100) -> None:
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
        assert pynvml is not None
        pynvml.nvmlInit()
        node_id = platform.node()
        count = pynvml.nvmlDeviceGetCount()

        cuda_idx = 0
        for i in range(count):
            handle = pynvml.nvmlDeviceGetHandleByIndex(i)
            gpu_uuid: str = pynvml.nvmlDeviceGetUUID(handle)
            model: str = pynvml.nvmlDeviceGetName(handle)
            mem_info = pynvml.nvmlDeviceGetMemoryInfo(handle)
            mem_total_gb = mem_info.total / (1024**3)

            mig_enabled = False
            try:
                mig_mode, _ = pynvml.nvmlDeviceGetMigMode(handle)
                if mig_mode == pynvml.NVML_DEVICE_MIG_ENABLE:
                    mig_enabled = True
                    max_mig = 7
                    for j in range(max_mig):
                        try:
                            mig_handle = pynvml.nvmlDeviceGetMigDeviceHandleByIndex(handle, j)
                            mig_uuid: str = pynvml.nvmlDeviceGetUUID(mig_handle)
                            mig_mem = pynvml.nvmlDeviceGetMemoryInfo(mig_handle)
                            mig_mem_gb = mig_mem.total / (1024**3)
                            resource_type = f"mig_{int(mig_mem_gb)}gb"
                            label = f"cuda:{cuda_idx}"
                            self._register_resource(
                                mig_uuid, gpu_uuid, resource_type, label,
                                model, node_id, mig_mem_gb, mig_handle,
                            )
                            cuda_idx += 1
                        except pynvml.NVMLError:
                            break
            except pynvml.NVMLError:
                pass

            if not mig_enabled:
                label = f"cuda:{cuda_idx}"
                self._register_resource(
                    gpu_uuid, gpu_uuid, "full_gpu", label,
                    model, node_id, mem_total_gb, handle,
                )
                cuda_idx += 1

    def _register_resource(
        self,
        resource_uuid: str,
        physical_uuid: str,
        resource_type: str,
        label: str,
        model: str,
        node_id: str,
        memory_total_gb: float,
        handle: object,
    ) -> None:
        self._label_to_resource[label] = resource_uuid
        self._resource_to_physical[resource_uuid] = physical_uuid
        self._gpu_info[resource_uuid] = _GPUStaticInfo(
            model=model,
            node_id=node_id,
            resource_type=resource_type,
            physical_gpu_uuid=physical_uuid,
            memory_total_gb=memory_total_gb,
        )
        self._handles[resource_uuid] = handle
        self._snapshots[resource_uuid] = _GPUSnapshot(
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
        try:
            assert pynvml is not None
            pynvml.nvmlShutdown()
        except Exception:  # noqa: BLE001
            pass

    def _collection_loop(self) -> None:
        assert pynvml is not None
        while not self._stop_event.wait(self._interval_s):
            for resource_uuid, handle in self._handles.items():
                try:
                    self._snapshots[resource_uuid] = self._collect_one(handle)
                except Exception:  # noqa: BLE001
                    pass

    def _collect_one(self, handle: object) -> _GPUSnapshot:
        assert pynvml is not None
        mem = pynvml.nvmlDeviceGetMemoryInfo(handle)
        util = pynvml.nvmlDeviceGetUtilizationRates(handle)
        temp = pynvml.nvmlDeviceGetTemperature(handle, pynvml.NVML_TEMPERATURE_GPU)
        power = pynvml.nvmlDeviceGetPowerUsage(handle) // 1000  # mW → W
        clock = pynvml.nvmlDeviceGetClockInfo(handle, pynvml.NVML_CLOCK_SM)
        return _GPUSnapshot(
            memory_used_gb=mem.used / (1024**3),
            utilization=float(util.gpu),
            temperature_celsius=temp,
            power_watts=power,
            clock_mhz=clock,
        )

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


class MockGPUProfiler:
    """Test-only profiler that simulates GPU discovery and metrics without pynvml."""

    def __init__(self, *, num_gpus: int = 2, mig_enabled: bool = False) -> None:
        self._label_to_resource: dict[str, str] = {}
        self._resource_to_physical: dict[str, str] = {}
        self._snapshots: dict[str, _GPUSnapshot] = {}
        self._gpu_info: dict[str, _GPUStaticInfo] = {}

        node_id = "test-node"
        cuda_idx = 0

        for i in range(num_gpus):
            gpu_uuid = f"GPU-{i:04d}"

            if mig_enabled:
                for j in range(2):
                    mig_uuid = f"MIG-{i:04d}-{j:02d}"
                    label = f"cuda:{cuda_idx}"
                    self._label_to_resource[label] = mig_uuid
                    self._resource_to_physical[mig_uuid] = gpu_uuid
                    self._gpu_info[mig_uuid] = _GPUStaticInfo(
                        model="NVIDIA H100 80GB HBM3",
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
                label = f"cuda:{cuda_idx}"
                self._label_to_resource[label] = gpu_uuid
                self._resource_to_physical[gpu_uuid] = gpu_uuid
                self._gpu_info[gpu_uuid] = _GPUStaticInfo(
                    model="NVIDIA H100 80GB HBM3",
                    node_id=node_id,
                    resource_type="full_gpu",
                    physical_gpu_uuid=gpu_uuid,
                    memory_total_gb=80.0,
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


def create_gpu_profiler(
    *, snapshot_interval_ms: int = 100
) -> GPUProfiler | MockGPUProfiler | None:
    """Factory: returns a real GPUProfiler if pynvml is available, else None."""
    if not _HAS_PYNVML:
        logger.info("pynvml not available — GPU profiling disabled")
        return None
    try:
        return GPUProfiler(snapshot_interval_ms=snapshot_interval_ms)
    except Exception:  # noqa: BLE001
        logger.info("GPU discovery failed — GPU profiling disabled", exc_info=True)
        return None
