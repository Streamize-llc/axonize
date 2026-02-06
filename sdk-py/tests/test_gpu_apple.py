"""Tests for Apple Silicon GPU backend — requires macOS ARM64."""

from __future__ import annotations

import platform

import pytest

pytestmark = pytest.mark.skipif(
    platform.system() != "Darwin" or platform.machine() != "arm64",
    reason="Apple Silicon required",
)


class TestAppleSiliconDiscovery:
    def test_discover_returns_one_gpu(self) -> None:
        from axonize._gpu_apple import AppleSiliconBackend

        backend = AppleSiliconBackend()
        gpus = backend.discover()
        assert len(gpus) == 1

    def test_discover_vendor_is_apple(self) -> None:
        from axonize._gpu_apple import AppleSiliconBackend

        backend = AppleSiliconBackend()
        gpu = backend.discover()[0]
        assert gpu.vendor == "Apple"

    def test_discover_label_is_mps0(self) -> None:
        from axonize._gpu_apple import AppleSiliconBackend

        backend = AppleSiliconBackend()
        gpu = backend.discover()[0]
        assert gpu.label == "mps:0"

    def test_discover_resource_type_full_gpu(self) -> None:
        from axonize._gpu_apple import AppleSiliconBackend

        backend = AppleSiliconBackend()
        gpu = backend.discover()[0]
        assert gpu.resource_type == "full_gpu"

    def test_discover_model_contains_apple(self) -> None:
        from axonize._gpu_apple import AppleSiliconBackend

        backend = AppleSiliconBackend()
        gpu = backend.discover()[0]
        assert "Apple" in gpu.model

    def test_discover_stable_uuid(self) -> None:
        """UUID should be deterministic for the same machine."""
        from axonize._gpu_apple import AppleSiliconBackend

        backend1 = AppleSiliconBackend()
        backend2 = AppleSiliconBackend()
        assert backend1.discover()[0].resource_uuid == backend2.discover()[0].resource_uuid

    def test_discover_uuid_starts_with_apple(self) -> None:
        from axonize._gpu_apple import AppleSiliconBackend

        backend = AppleSiliconBackend()
        uuid = backend.discover()[0].resource_uuid
        assert uuid.startswith("APPLE-")
        assert len(uuid) == 18  # "APPLE-" + 12 hex chars

    def test_memory_total_reasonable(self) -> None:
        """Unified memory should be in a sane range (4-256 GB)."""
        from axonize._gpu_apple import AppleSiliconBackend

        backend = AppleSiliconBackend()
        gpu = backend.discover()[0]
        assert 4.0 <= gpu.memory_total_gb <= 256.0


class TestAppleSiliconCollect:
    def test_collect_returns_snapshot(self) -> None:
        from axonize._gpu_apple import AppleSiliconBackend
        from axonize._gpu_backend import _GPUSnapshot

        backend = AppleSiliconBackend()
        snapshot = backend.collect(None)
        assert isinstance(snapshot, _GPUSnapshot)

    def test_collect_utilization_in_range(self) -> None:
        from axonize._gpu_apple import AppleSiliconBackend

        backend = AppleSiliconBackend()
        snapshot = backend.collect(None)
        assert 0.0 <= snapshot.utilization <= 100.0

    def test_collect_memory_used_non_negative(self) -> None:
        from axonize._gpu_apple import AppleSiliconBackend

        backend = AppleSiliconBackend()
        snapshot = backend.collect(None)
        assert snapshot.memory_used_gb >= 0.0

    def test_collect_power_non_negative(self) -> None:
        from axonize._gpu_apple import AppleSiliconBackend

        backend = AppleSiliconBackend()
        snapshot = backend.collect(None)
        assert snapshot.power_watts >= 0


class TestAppleSiliconWithProfiler:
    def test_profiler_integration(self) -> None:
        """AppleSiliconBackend works through the GPUProfiler."""
        from axonize._gpu import GPUProfiler
        from axonize._gpu_apple import AppleSiliconBackend

        backend = AppleSiliconBackend()
        profiler = GPUProfiler(backend=backend, snapshot_interval_ms=100)
        labels = profiler.resolve_labels(["mps:0"])
        assert len(labels) == 1
        assert labels[0].vendor == "Apple"
        assert labels[0].user_label == "mps:0"
        profiler.stop()

    def test_unknown_label_returns_empty(self) -> None:
        from axonize._gpu import GPUProfiler
        from axonize._gpu_apple import AppleSiliconBackend

        backend = AppleSiliconBackend()
        profiler = GPUProfiler(backend=backend, snapshot_interval_ms=100)
        labels = profiler.resolve_labels(["cuda:0"])
        assert labels == []
        profiler.stop()


class TestEnergyRawToJoules:
    """Unit conversion tests — run on any platform (no IOKit needed)."""

    pytestmark = []  # Override module-level skipif

    def test_millijoules(self) -> None:
        from axonize._gpu_apple import _IOReportSampler

        assert _IOReportSampler._energy_raw_to_joules(1000, "mJ") == pytest.approx(1.0)

    def test_microjoules(self) -> None:
        from axonize._gpu_apple import _IOReportSampler

        assert _IOReportSampler._energy_raw_to_joules(1_000_000, "uJ") == pytest.approx(1.0)

    def test_microjoules_unicode(self) -> None:
        from axonize._gpu_apple import _IOReportSampler

        assert _IOReportSampler._energy_raw_to_joules(1_000_000, "\u00b5J") == pytest.approx(1.0)

    def test_nanojoules(self) -> None:
        from axonize._gpu_apple import _IOReportSampler

        assert _IOReportSampler._energy_raw_to_joules(1_000_000_000, "nJ") == pytest.approx(1.0)

    def test_unknown_unit_assumes_millijoules(self) -> None:
        from axonize._gpu_apple import _IOReportSampler

        assert _IOReportSampler._energy_raw_to_joules(500, "") == pytest.approx(0.5)

    def test_watts_from_energy_and_time(self) -> None:
        """100ms interval, 1000 mJ → 10 W."""
        from axonize._gpu_apple import _IOReportSampler

        joules = _IOReportSampler._energy_raw_to_joules(1000, "mJ")
        watts = joules / 0.1  # 100ms
        assert watts == pytest.approx(10.0)


class TestAppleSiliconShutdown:
    def test_shutdown_idempotent(self) -> None:
        from axonize._gpu_apple import AppleSiliconBackend

        backend = AppleSiliconBackend()
        backend.shutdown()
        backend.shutdown()  # Should not raise
