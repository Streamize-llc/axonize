"""Apple Silicon GPU backend — IOKit-based metrics via ctypes."""

from __future__ import annotations

import ctypes
import ctypes.util
import hashlib
import logging
import platform
import subprocess
import time
from typing import Any

from axonize._gpu_backend import DiscoveredGPU, _GPUSnapshot

logger = logging.getLogger("axonize.gpu.apple")


def _sysctl_str(name: str) -> str:
    """Read a sysctl string value."""
    result = subprocess.run(
        ["sysctl", "-n", name],  # noqa: S603, S607
        capture_output=True,
        text=True,
        timeout=5,
    )
    return result.stdout.strip()


def _sysctl_int(name: str) -> int:
    """Read a sysctl integer value."""
    return int(_sysctl_str(name))


# --- IOKit ctypes bindings ---

_iokit: ctypes.CDLL | None = None
_cf: ctypes.CDLL | None = None


def _load_iokit() -> tuple[ctypes.CDLL, ctypes.CDLL]:
    """Load IOKit and CoreFoundation frameworks."""
    global _iokit, _cf  # noqa: PLW0603
    if _iokit is not None and _cf is not None:
        return _iokit, _cf

    iokit_path = ctypes.util.find_library("IOKit")
    cf_path = ctypes.util.find_library("CoreFoundation")
    if not iokit_path or not cf_path:
        raise RuntimeError("IOKit or CoreFoundation not found")

    _iokit = ctypes.CDLL(iokit_path)
    _cf = ctypes.CDLL(cf_path)
    return _iokit, _cf


def _cfstr(cf: ctypes.CDLL, s: str) -> ctypes.c_void_p:
    """Create a CFStringRef from a Python string."""
    cf.CFStringCreateWithCString.restype = ctypes.c_void_p
    cf.CFStringCreateWithCString.argtypes = [
        ctypes.c_void_p,
        ctypes.c_char_p,
        ctypes.c_uint32,
    ]
    # kCFStringEncodingUTF8 = 0x08000100
    result: ctypes.c_void_p = cf.CFStringCreateWithCString(None, s.encode("utf-8"), 0x08000100)
    return result


def _cf_release(cf: ctypes.CDLL, ref: ctypes.c_void_p) -> None:
    """Release a CoreFoundation object."""
    if ref:
        cf.CFRelease.argtypes = [ctypes.c_void_p]
        cf.CFRelease(ref)


class _IOReportSampler:
    """Thin wrapper around IOReport APIs for GPU metrics sampling."""

    def __init__(self) -> None:
        self._iokit, self._cf = _load_iokit()

        # Set up IOReportCopyChannelsInGroup
        self._iokit.IOReportCopyChannelsInGroup.restype = ctypes.c_void_p
        self._iokit.IOReportCopyChannelsInGroup.argtypes = [
            ctypes.c_void_p,  # group name (CFStringRef)
            ctypes.c_void_p,  # subgroup (NULL for all)
            ctypes.c_uint64,  # unused
            ctypes.c_uint64,  # unused
            ctypes.c_uint64,  # unused
        ]

        # IOReportCreateSubscription
        self._iokit.IOReportCreateSubscription.restype = ctypes.c_void_p
        self._iokit.IOReportCreateSubscription.argtypes = [
            ctypes.c_void_p,  # driver (NULL)
            ctypes.c_void_p,  # channels (CFDictionary)
            ctypes.c_void_p,  # out subscription
            ctypes.c_uint64,  # unused
            ctypes.c_void_p,  # unused
        ]

        # IOReportCreateSamples / IOReportCreateSamplesDelta
        self._iokit.IOReportCreateSamples.restype = ctypes.c_void_p
        self._iokit.IOReportCreateSamples.argtypes = [
            ctypes.c_void_p,  # subscription
            ctypes.c_void_p,  # channels (CFDictionary)
            ctypes.c_void_p,  # unused
        ]

        self._iokit.IOReportCreateSamplesDelta.restype = ctypes.c_void_p
        self._iokit.IOReportCreateSamplesDelta.argtypes = [
            ctypes.c_void_p,  # prev
            ctypes.c_void_p,  # curr
            ctypes.c_void_p,  # unused
        ]

        # Channel iteration
        self._iokit.IOReportIterate.restype = None
        self._iokit.IOReportIterate.argtypes = [
            ctypes.c_void_p,  # samples
            ctypes.CFUNCTYPE(ctypes.c_int, ctypes.c_void_p),  # callback
        ]

        self._iokit.IOReportChannelGetGroup.restype = ctypes.c_void_p
        self._iokit.IOReportChannelGetGroup.argtypes = [ctypes.c_void_p]

        self._iokit.IOReportChannelGetSubGroup.restype = ctypes.c_void_p
        self._iokit.IOReportChannelGetSubGroup.argtypes = [ctypes.c_void_p]

        self._iokit.IOReportChannelGetChannelName.restype = ctypes.c_void_p
        self._iokit.IOReportChannelGetChannelName.argtypes = [ctypes.c_void_p]

        self._iokit.IOReportSimpleGetIntegerValue.restype = ctypes.c_int64
        self._iokit.IOReportSimpleGetIntegerValue.argtypes = [
            ctypes.c_void_p,
            ctypes.c_int,
        ]

        self._iokit.IOReportStateGetCount.restype = ctypes.c_int
        self._iokit.IOReportStateGetCount.argtypes = [ctypes.c_void_p]

        self._iokit.IOReportStateGetResidency.restype = ctypes.c_int64
        self._iokit.IOReportStateGetResidency.argtypes = [
            ctypes.c_void_p,
            ctypes.c_int,
        ]

        self._iokit.IOReportChannelGetUnitLabel.restype = ctypes.c_void_p
        self._iokit.IOReportChannelGetUnitLabel.argtypes = [ctypes.c_void_p]

        # Build channel subscription for GPU-related groups
        self._subscription = None
        self._channels = None
        self._prev_sample: ctypes.c_void_p | None = None
        self._prev_sample_time: float = 0.0

        self._init_subscription()

    def _init_subscription(self) -> None:
        """Subscribe to GPU Energy Model and GPU Performance States channels."""
        groups = ["Energy Model", "GPU Performance States"]
        all_channels = None

        for group_name in groups:
            group_str = _cfstr(self._cf, group_name)
            try:
                channels = self._iokit.IOReportCopyChannelsInGroup(
                    group_str, None, 0, 0, 0
                )
                if channels:
                    if all_channels is None:
                        all_channels = channels
                    else:
                        # Merge channels
                        self._iokit.IOReportMergeChannels.restype = None
                        self._iokit.IOReportMergeChannels.argtypes = [
                            ctypes.c_void_p,
                            ctypes.c_void_p,
                            ctypes.c_void_p,
                        ]
                        self._iokit.IOReportMergeChannels(all_channels, channels, None)
            finally:
                _cf_release(self._cf, group_str)

        if all_channels is None:
            raise RuntimeError("No IOReport GPU channels found")

        self._channels = all_channels

        # Create subscription
        sub_out = ctypes.c_void_p()
        self._subscription = self._iokit.IOReportCreateSubscription(
            None, all_channels, ctypes.byref(sub_out), 0, None
        )

    def _cfstring_to_str(self, cfstr: ctypes.c_void_p) -> str:
        """Convert a CFStringRef to a Python str."""
        if not cfstr:
            return ""
        buf = ctypes.create_string_buffer(256)
        self._cf.CFStringGetCString.restype = ctypes.c_bool
        self._cf.CFStringGetCString.argtypes = [
            ctypes.c_void_p,
            ctypes.c_char_p,
            ctypes.c_long,
            ctypes.c_uint32,
        ]
        if self._cf.CFStringGetCString(cfstr, buf, 256, 0x08000100):
            return buf.value.decode("utf-8", errors="replace")
        return ""

    @staticmethod
    def _energy_raw_to_joules(raw: int, unit: str) -> float:
        """Convert IOReport raw energy value to joules based on unit label.

        IOReport channels may report energy in mJ, uJ, or nJ.
        Reference: macmon by vladkens (https://github.com/vladkens/macmon)
        """
        if unit.startswith("m"):  # mJ
            return raw / 1e3
        if unit.startswith("u") or unit.startswith("\u00b5"):  # uJ or µJ
            return raw / 1e6
        if unit.startswith("n"):  # nJ
            return raw / 1e9
        # Unknown unit — assume mJ (most common on Apple Silicon)
        return raw / 1e3

    def sample(self) -> dict[str, float]:
        """Take a sample and return GPU metrics as a flat dict.

        Keys: gpu_power_w (watts), gpu_active_residency_pct (0-100)
        """
        if self._subscription is None:
            return {}

        curr_time = time.monotonic()
        curr_sample = self._iokit.IOReportCreateSamples(
            self._subscription, self._channels, None
        )
        if not curr_sample:
            return {}

        if self._prev_sample is None:
            self._prev_sample = curr_sample
            self._prev_sample_time = curr_time
            # Need two samples for delta-based metrics — return zeros on first call
            return {}

        delta_s = curr_time - self._prev_sample_time
        if delta_s <= 0:
            _cf_release(self._cf, curr_sample)
            return {}

        delta = self._iokit.IOReportCreateSamplesDelta(
            self._prev_sample, curr_sample, None
        )
        _cf_release(self._cf, self._prev_sample)
        self._prev_sample = curr_sample
        self._prev_sample_time = curr_time

        if not delta:
            return {}

        metrics: dict[str, float] = {}
        # Capture delta_s in closure for energy→power conversion
        sample_delta_s = delta_s
        sampler_self = self

        @ctypes.CFUNCTYPE(ctypes.c_int, ctypes.c_void_p)
        def _iterate_cb(entry: ctypes.c_void_p) -> int:
            group = sampler_self._cfstring_to_str(
                sampler_self._iokit.IOReportChannelGetGroup(entry)
            )
            channel = sampler_self._cfstring_to_str(
                sampler_self._iokit.IOReportChannelGetChannelName(entry)
            )

            # GPU power from Energy Model: energy → watts
            if "Energy Model" in group and "GPU" in channel:
                raw = sampler_self._iokit.IOReportSimpleGetIntegerValue(entry, 0)
                unit = sampler_self._cfstring_to_str(
                    sampler_self._iokit.IOReportChannelGetUnitLabel(entry)
                )
                joules = sampler_self._energy_raw_to_joules(raw, unit)
                watts = joules / sample_delta_s
                # Accumulate if multiple GPU energy channels exist
                metrics["gpu_power_w"] = metrics.get("gpu_power_w", 0.0) + watts

            # GPU utilization from Performance States
            if "GPU Performance States" in group:
                state_count = sampler_self._iokit.IOReportStateGetCount(entry)
                total_residency = 0
                active_residency = 0
                for s in range(state_count):
                    residency = sampler_self._iokit.IOReportStateGetResidency(entry, s)
                    total_residency += residency
                    if s > 0:  # State 0 is usually idle
                        active_residency += residency
                if total_residency > 0:
                    metrics["gpu_active_residency_pct"] = (
                        active_residency / total_residency * 100.0
                    )

            return 0  # kIOReportIterOk

        self._iokit.IOReportIterate(delta, _iterate_cb)
        _cf_release(self._cf, delta)

        return metrics


class AppleSiliconBackend:
    """Apple Silicon GPU backend using IOKit for metrics."""

    vendor = "Apple"

    def __init__(self) -> None:
        if platform.system() != "Darwin" or platform.machine() != "arm64":
            raise RuntimeError("Apple Silicon backend requires macOS on ARM64")

        # Cache chip info
        self._chip_model = _sysctl_str("machdep.cpu.brand_string")
        self._memory_total_bytes = _sysctl_int("hw.memsize")
        self._memory_total_gb = self._memory_total_bytes / (1024**3)
        self._node_id = platform.node()

        # Generate deterministic UUID: APPLE-{sha256(chip+hostname)[:12]}
        # WARNING: UUID changes if hostname changes (e.g. DHCP rename).
        # For stable identity in dynamic environments, consider an external
        # machine-id source in the future.
        hash_input = f"{self._chip_model}:{self._node_id}"
        self._gpu_uuid = "APPLE-" + hashlib.sha256(hash_input.encode()).hexdigest()[:12]

        # IOKit sampler — may fail if IOReport not available
        self._sampler: _IOReportSampler | None = None
        try:
            self._sampler = _IOReportSampler()
            # Take initial sample to prime the delta
            self._sampler.sample()
        except Exception:  # noqa: BLE001
            logger.debug("IOReport sampling unavailable, metrics will be zeros", exc_info=True)

    def discover(self) -> list[DiscoveredGPU]:
        return [DiscoveredGPU(
            resource_uuid=self._gpu_uuid,
            physical_gpu_uuid=self._gpu_uuid,
            resource_type="full_gpu",
            label="mps:0",
            model=self._chip_model,
            vendor=self.vendor,
            node_id=self._node_id,
            memory_total_gb=self._memory_total_gb,
            handle=None,  # Apple Silicon has no per-device handle
        )]

    def collect(self, handle: Any) -> _GPUSnapshot:
        metrics: dict[str, float] = {}
        if self._sampler is not None:
            try:
                metrics = self._sampler.sample()
            except Exception:  # noqa: BLE001
                pass

        utilization = metrics.get("gpu_active_residency_pct", 0.0)
        utilization = max(0.0, min(100.0, utilization))

        # gpu_power_w is already converted: E(J) / t(s) = W
        power_w = int(round(metrics.get("gpu_power_w", 0.0)))
        power_w = max(0, power_w)

        return _GPUSnapshot(
            memory_used_gb=0.0,
            utilization=utilization,
            temperature_celsius=0,  # Not available via IOKit reliably
            power_watts=power_w,
            clock_mhz=0,  # Not available via IOKit reliably
        )

    def shutdown(self) -> None:
        self._sampler = None
