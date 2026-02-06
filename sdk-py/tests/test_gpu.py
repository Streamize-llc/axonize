"""Tests for GPU profiler — mock-based (no real GPU required)."""

from __future__ import annotations

import time

import pytest

from axonize._gpu import MockGPUProfiler, create_gpu_profiler
from axonize._types import GPUAttribution


class TestMockProfilerDiscovery:
    def test_two_full_gpus(self) -> None:
        profiler = MockGPUProfiler(num_gpus=2)
        assert len(profiler._label_to_resource) == 2
        assert "cuda:0" in profiler._label_to_resource
        assert "cuda:1" in profiler._label_to_resource

    def test_single_gpu(self) -> None:
        profiler = MockGPUProfiler(num_gpus=1)
        assert len(profiler._label_to_resource) == 1
        assert "cuda:0" in profiler._label_to_resource

    def test_resource_type_full_gpu(self) -> None:
        profiler = MockGPUProfiler(num_gpus=1)
        uuid = profiler._label_to_resource["cuda:0"]
        assert profiler._gpu_info[uuid].resource_type == "full_gpu"

    def test_physical_uuid_equals_resource_for_full_gpu(self) -> None:
        profiler = MockGPUProfiler(num_gpus=1)
        uuid = profiler._label_to_resource["cuda:0"]
        assert profiler._resource_to_physical[uuid] == uuid


class TestMockProfilerMIG:
    def test_mig_discovery(self) -> None:
        profiler = MockGPUProfiler(num_gpus=2, mig_enabled=True)
        # 2 GPUs * 2 MIG instances each = 4 cuda devices
        assert len(profiler._label_to_resource) == 4
        for i in range(4):
            assert f"cuda:{i}" in profiler._label_to_resource

    def test_mig_resource_type(self) -> None:
        profiler = MockGPUProfiler(num_gpus=1, mig_enabled=True)
        uuid = profiler._label_to_resource["cuda:0"]
        assert profiler._gpu_info[uuid].resource_type == "mig_40gb"

    def test_mig_physical_differs_from_resource(self) -> None:
        profiler = MockGPUProfiler(num_gpus=1, mig_enabled=True)
        mig_uuid = profiler._label_to_resource["cuda:0"]
        physical_uuid = profiler._resource_to_physical[mig_uuid]
        assert mig_uuid != physical_uuid
        assert mig_uuid.startswith("MIG-")
        assert physical_uuid.startswith("GPU-")


class TestResolveLabels:
    def test_resolve_single_label(self) -> None:
        profiler = MockGPUProfiler(num_gpus=2)
        result = profiler.resolve_labels(["cuda:0"])
        assert len(result) == 1
        attr = result[0]
        assert isinstance(attr, GPUAttribution)
        assert attr.user_label == "cuda:0"
        assert attr.resource_type == "full_gpu"
        assert attr.gpu_model == "NVIDIA H100 80GB HBM3"
        assert attr.memory_total_gb == 80.0
        assert attr.utilization >= 0.0

    def test_resolve_multiple_labels(self) -> None:
        profiler = MockGPUProfiler(num_gpus=2)
        result = profiler.resolve_labels(["cuda:0", "cuda:1"])
        assert len(result) == 2
        assert result[0].user_label == "cuda:0"
        assert result[1].user_label == "cuda:1"

    def test_resolve_unknown_label(self) -> None:
        profiler = MockGPUProfiler(num_gpus=1)
        result = profiler.resolve_labels(["cuda:99"])
        assert result == []

    def test_resolve_mixed_known_unknown(self) -> None:
        profiler = MockGPUProfiler(num_gpus=1)
        result = profiler.resolve_labels(["cuda:0", "cuda:99"])
        assert len(result) == 1
        assert result[0].user_label == "cuda:0"

    def test_attribution_fields_complete(self) -> None:
        profiler = MockGPUProfiler(num_gpus=1)
        attr = profiler.resolve_labels(["cuda:0"])[0]
        assert attr.resource_uuid == "GPU-0000"
        assert attr.physical_gpu_uuid == "GPU-0000"
        assert attr.node_id == "test-node"
        assert attr.memory_used_gb > 0
        assert attr.temperature_celsius > 0
        assert attr.power_watts > 0
        assert attr.clock_mhz > 0


class TestMockProfilerLifecycle:
    def test_start_stop_noop(self) -> None:
        profiler = MockGPUProfiler(num_gpus=1)
        profiler.start()
        profiler.stop()

    def test_snapshots_present_at_init(self) -> None:
        profiler = MockGPUProfiler(num_gpus=2)
        assert len(profiler._snapshots) == 2


class TestGracefulDegradation:
    def test_create_profiler_no_pynvml(self, monkeypatch: pytest.MonkeyPatch) -> None:
        import axonize._gpu as gpu_mod

        monkeypatch.setattr(gpu_mod, "_HAS_PYNVML", False)
        result = create_gpu_profiler()
        assert result is None


class TestSpanSetGpusWithProfiler:
    def test_span_attributions_populated(self) -> None:
        import axonize._sdk as sdk_mod

        mock_profiler = MockGPUProfiler(num_gpus=2)
        original = sdk_mod._sdk_instance

        try:
            # Create a minimal SDK with a mock profiler
            from axonize._config import AxonizeConfig

            config = AxonizeConfig(
                endpoint="localhost:4317",
                service_name="test",
                gpu_profiling=True,
            )
            fake_sdk = sdk_mod._AxonizeSDK(config)
            fake_sdk._gpu_profiler = mock_profiler
            sdk_mod._sdk_instance = fake_sdk

            from axonize._buffer import RingBuffer
            from axonize._span import Span

            buf = RingBuffer(16)
            span = Span("test-op", buffer=buf, service_name="test")
            span.set_gpus(["cuda:0", "cuda:1"])

            assert len(span._gpu_attributions) == 2
            assert span._gpu_attributions[0].user_label == "cuda:0"
            assert span._gpu_attributions[1].user_label == "cuda:1"
        finally:
            sdk_mod._sdk_instance = original

    def test_span_attributions_empty_without_profiler(self) -> None:
        import axonize._sdk as sdk_mod

        original = sdk_mod._sdk_instance
        try:
            sdk_mod._sdk_instance = None

            from axonize._buffer import RingBuffer
            from axonize._span import Span

            buf = RingBuffer(16)
            span = Span("test-op", buffer=buf, service_name="test")
            span.set_gpus(["cuda:0"])

            assert span._gpu_attributions == []
        finally:
            sdk_mod._sdk_instance = original


class TestOverheadBenchmark:
    def test_resolve_labels_fast(self) -> None:
        profiler = MockGPUProfiler(num_gpus=4)
        labels = ["cuda:0", "cuda:1"]

        # Warmup
        for _ in range(1000):
            profiler.resolve_labels(labels)

        iterations = 100_000
        start = time.perf_counter_ns()
        for _ in range(iterations):
            profiler.resolve_labels(labels)
        elapsed_ns = time.perf_counter_ns() - start

        ns_per_call = elapsed_ns / iterations
        # Target < 5μs (5000ns) per call — includes frozen dataclass construction.
        # The actual dict lookups are < 100ns; object creation dominates in CPython.
        assert ns_per_call < 5000, f"resolve_labels too slow: {ns_per_call:.0f}ns/call"
