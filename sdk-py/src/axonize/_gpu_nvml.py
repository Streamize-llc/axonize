"""NVIDIA pynvml GPU backend — extracted from the original GPUProfiler."""

from __future__ import annotations

import logging
import platform
import warnings
from typing import Any

from axonize._gpu_backend import DiscoveredGPU, _GPUSnapshot

logger = logging.getLogger("axonize.gpu.nvml")

# pynvml is optional — imported at class init time.
warnings.filterwarnings("ignore", category=FutureWarning, message=".*pynvml.*deprecated.*")
try:
    import pynvml

    _HAS_PYNVML = True
except ImportError:
    pynvml = None  # type: ignore[assignment,unused-ignore]
    _HAS_PYNVML = False


class NvmlBackend:
    """NVIDIA GPU backend using pynvml."""

    vendor = "NVIDIA"

    def __init__(self) -> None:
        if not _HAS_PYNVML:
            raise RuntimeError("pynvml is not installed")
        assert pynvml is not None
        pynvml.nvmlInit()

    def discover(self) -> list[DiscoveredGPU]:
        assert pynvml is not None
        node_id = platform.node()
        count = pynvml.nvmlDeviceGetCount()
        gpus: list[DiscoveredGPU] = []

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
                            gpus.append(DiscoveredGPU(
                                resource_uuid=mig_uuid,
                                physical_gpu_uuid=gpu_uuid,
                                resource_type=resource_type,
                                label=label,
                                model=model,
                                vendor=self.vendor,
                                node_id=node_id,
                                memory_total_gb=mig_mem_gb,
                                handle=mig_handle,
                            ))
                            cuda_idx += 1
                        except pynvml.NVMLError:
                            break
            except pynvml.NVMLError:
                pass

            if not mig_enabled:
                label = f"cuda:{cuda_idx}"
                gpus.append(DiscoveredGPU(
                    resource_uuid=gpu_uuid,
                    physical_gpu_uuid=gpu_uuid,
                    resource_type="full_gpu",
                    label=label,
                    model=model,
                    vendor=self.vendor,
                    node_id=node_id,
                    memory_total_gb=mem_total_gb,
                    handle=handle,
                ))
                cuda_idx += 1

        return gpus

    def collect(self, handle: Any) -> _GPUSnapshot:
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

    def shutdown(self) -> None:
        try:
            assert pynvml is not None
            pynvml.nvmlShutdown()
        except Exception:  # noqa: BLE001
            pass
