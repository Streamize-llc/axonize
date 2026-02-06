"""Tests for GPU backend abstraction — works on any platform."""

from __future__ import annotations

from typing import Any

from axonize._gpu import GPUProfiler, MockGPUProfiler
from axonize._gpu_backend import DiscoveredGPU, GPUBackend, _GPUSnapshot
from axonize._types import GPUAttribution


class _FakeBackend:
    """Minimal backend for testing the protocol."""

    vendor = "TestVendor"

    def __init__(self, gpus: list[DiscoveredGPU] | None = None) -> None:
        self._gpus = gpus or []
        self._shutdown_called = False

    def discover(self) -> list[DiscoveredGPU]:
        return self._gpus

    def collect(self, handle: Any) -> _GPUSnapshot:
        return _GPUSnapshot(
            memory_used_gb=10.0,
            utilization=50.0,
            temperature_celsius=60,
            power_watts=100,
            clock_mhz=1200,
        )

    def shutdown(self) -> None:
        self._shutdown_called = True


class TestGPUBackendProtocol:
    def test_fake_backend_satisfies_protocol(self) -> None:
        backend = _FakeBackend()
        assert isinstance(backend, GPUBackend)

    def test_protocol_requires_vendor(self) -> None:
        class _NoVendor:
            def discover(self) -> list[DiscoveredGPU]:
                return []

            def collect(self, handle: Any) -> _GPUSnapshot:
                return _GPUSnapshot(0, 0, 0, 0, 0)

            def shutdown(self) -> None:
                pass

        assert not isinstance(_NoVendor(), GPUBackend)


class TestGPUProfilerWithBackend:
    def _make_backend(self, *, num_gpus: int = 1) -> _FakeBackend:
        gpus = []
        for i in range(num_gpus):
            gpus.append(DiscoveredGPU(
                resource_uuid=f"TEST-{i:04d}",
                physical_gpu_uuid=f"TEST-{i:04d}",
                resource_type="full_gpu",
                label=f"test:{i}",
                model="Test GPU 16GB",
                vendor="TestVendor",
                node_id="test-host",
                memory_total_gb=16.0,
                handle=i,
            ))
        return _FakeBackend(gpus)

    def test_discover_populates_labels(self) -> None:
        backend = self._make_backend(num_gpus=2)
        profiler = GPUProfiler(backend=backend, snapshot_interval_ms=100)
        assert "test:0" in profiler._label_to_resource
        assert "test:1" in profiler._label_to_resource

    def test_resolve_labels_returns_attribution(self) -> None:
        backend = self._make_backend()
        profiler = GPUProfiler(backend=backend, snapshot_interval_ms=100)
        result = profiler.resolve_labels(["test:0"])
        assert len(result) == 1
        assert isinstance(result[0], GPUAttribution)

    def test_vendor_in_attribution(self) -> None:
        backend = self._make_backend()
        profiler = GPUProfiler(backend=backend, snapshot_interval_ms=100)
        attr = profiler.resolve_labels(["test:0"])[0]
        assert attr.vendor == "TestVendor"

    def test_stop_calls_backend_shutdown(self) -> None:
        backend = self._make_backend()
        profiler = GPUProfiler(backend=backend, snapshot_interval_ms=100)
        profiler.stop()
        assert backend._shutdown_called

    def test_unknown_label_returns_empty(self) -> None:
        backend = self._make_backend()
        profiler = GPUProfiler(backend=backend, snapshot_interval_ms=100)
        assert profiler.resolve_labels(["nonexistent:0"]) == []


class TestVendorInOTLPExport:
    def test_vendor_attribute_in_otlp(self) -> None:
        from axonize._exporter import _span_data_to_otlp
        from axonize._types import SpanData, SpanKind, SpanStatus

        ga = GPUAttribution(
            resource_uuid="GPU-0000",
            physical_gpu_uuid="GPU-0000",
            gpu_model="NVIDIA H100 80GB HBM3",
            vendor="NVIDIA",
            node_id="worker-01",
            resource_type="full_gpu",
            user_label="cuda:0",
            memory_used_gb=42.5,
            memory_total_gb=80.0,
            utilization=85.2,
            temperature_celsius=72,
            power_watts=350,
            clock_mhz=1800,
        )
        sd = SpanData(
            span_id="abcdef0123456789",
            trace_id="0123456789abcdef0123456789abcdef",
            name="test",
            kind=SpanKind.INTERNAL,
            status=SpanStatus.OK,
            start_time_ns=1_000_000_000,
            end_time_ns=2_000_000_000,
            duration_ms=1000.0,
            service_name="test",
            gpu_attributions=[ga],
        )
        otlp = _span_data_to_otlp(sd)
        attr_dict = {a.key: a.value for a in otlp.attributes}
        assert "gpu.0.vendor" in attr_dict
        assert attr_dict["gpu.0.vendor"].string_value == "NVIDIA"

    def test_apple_vendor_in_otlp(self) -> None:
        from axonize._exporter import _span_data_to_otlp
        from axonize._types import SpanData, SpanKind, SpanStatus

        ga = GPUAttribution(
            resource_uuid="APPLE-abc123def456",
            physical_gpu_uuid="APPLE-abc123def456",
            gpu_model="Apple M3 Max",
            vendor="Apple",
            node_id="macbook",
            resource_type="full_gpu",
            user_label="mps:0",
            memory_used_gb=0.0,
            memory_total_gb=36.0,
            utilization=45.0,
            temperature_celsius=0,
            power_watts=15,
            clock_mhz=0,
        )
        sd = SpanData(
            span_id="abcdef0123456789",
            trace_id="0123456789abcdef0123456789abcdef",
            name="test",
            kind=SpanKind.INTERNAL,
            status=SpanStatus.OK,
            start_time_ns=1_000_000_000,
            end_time_ns=2_000_000_000,
            duration_ms=1000.0,
            service_name="test",
            gpu_attributions=[ga],
        )
        otlp = _span_data_to_otlp(sd)
        attr_dict = {a.key: a.value for a in otlp.attributes}
        assert attr_dict["gpu.0.vendor"].string_value == "Apple"
        assert attr_dict["gpu.0.model"].string_value == "Apple M3 Max"
        assert attr_dict["gpu.0.user_label"].string_value == "mps:0"


class TestMockGPUProfilerVendor:
    def test_default_vendor_nvidia(self) -> None:
        profiler = MockGPUProfiler(num_gpus=1)
        attr = profiler.resolve_labels(["cuda:0"])[0]
        assert attr.vendor == "NVIDIA"

    def test_apple_vendor(self) -> None:
        profiler = MockGPUProfiler(num_gpus=1, vendor="Apple")
        attr = profiler.resolve_labels(["mps:0"])[0]
        assert attr.vendor == "Apple"
        assert attr.gpu_model == "Apple M3 Max"
        assert attr.memory_total_gb == 36.0

    def test_apple_vendor_label_prefix(self) -> None:
        profiler = MockGPUProfiler(num_gpus=1, vendor="Apple")
        assert "mps:0" in profiler._label_to_resource
        assert "cuda:0" not in profiler._label_to_resource
