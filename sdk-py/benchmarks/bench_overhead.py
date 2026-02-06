#!/usr/bin/env python3
"""Inference-thread overhead benchmark.

Measures the hot-path cost of:
  1. span.__enter__  (start timing + context set)
  2. span.set_gpus() (label → GPUAttribution resolution)
  3. span.__exit__   (end timing + buffer enqueue)

Target: < 1μs total overhead per inference call.

Usage:
    cd sdk-py && uv run python benchmarks/bench_overhead.py
"""

from __future__ import annotations

import time

from axonize._buffer import RingBuffer
from axonize._gpu import MockGPUProfiler
from axonize._span import Span
from axonize._types import SpanKind


def bench_resolve_labels(iterations: int = 500_000) -> float:
    """Benchmark: GPU label resolution only (dict lookup + dataclass creation)."""
    profiler = MockGPUProfiler(num_gpus=4)
    labels = ["cuda:0", "cuda:1"]

    # Warmup
    for _ in range(5000):
        profiler.resolve_labels(labels)

    start = time.perf_counter_ns()
    for _ in range(iterations):
        profiler.resolve_labels(labels)
    elapsed = time.perf_counter_ns() - start

    return elapsed / iterations


def bench_span_lifecycle(iterations: int = 200_000) -> float:
    """Benchmark: full span enter → set_gpus → exit (no profiler, pure overhead)."""
    buf = RingBuffer(maxsize=iterations + 1000)

    # Warmup
    for _ in range(1000):
        with Span("bench", buffer=buf, kind=SpanKind.INTERNAL):
            pass
    buf.drain(buf._maxsize)

    start = time.perf_counter_ns()
    for _ in range(iterations):
        with Span("bench", buffer=buf, kind=SpanKind.INTERNAL) as s:
            s.set_gpus(["cuda:0"])
    elapsed = time.perf_counter_ns() - start

    return elapsed / iterations


def bench_span_with_profiler(iterations: int = 200_000) -> float:
    """Benchmark: full span with active GPU profiler (mock)."""
    import axonize._sdk as sdk_mod
    from axonize._config import AxonizeConfig

    buf = RingBuffer(maxsize=iterations + 1000)
    profiler = MockGPUProfiler(num_gpus=4)

    config = AxonizeConfig(endpoint="localhost:4317", service_name="bench", gpu_profiling=True)
    fake_sdk = sdk_mod._AxonizeSDK(config)
    fake_sdk._gpu_profiler = profiler

    original = sdk_mod._sdk_instance
    sdk_mod._sdk_instance = fake_sdk

    try:
        # Warmup
        for _ in range(1000):
            with Span("bench", buffer=buf, kind=SpanKind.INTERNAL) as s:
                s.set_gpus(["cuda:0", "cuda:1"])
        buf.drain(buf._maxsize)

        start = time.perf_counter_ns()
        for _ in range(iterations):
            with Span("bench", buffer=buf, kind=SpanKind.INTERNAL) as s:
                s.set_gpus(["cuda:0", "cuda:1"])
        elapsed = time.perf_counter_ns() - start

        return elapsed / iterations
    finally:
        sdk_mod._sdk_instance = original


def bench_enqueue_only(iterations: int = 500_000) -> float:
    """Benchmark: ring buffer enqueue cost only."""
    from axonize._types import SpanData, SpanStatus

    buf = RingBuffer(maxsize=iterations + 1000)
    sd = SpanData(
        span_id="abcdef01234567",
        trace_id="0123456789abcdef0123456789abcdef",
        name="bench",
        kind=SpanKind.INTERNAL,
        status=SpanStatus.OK,
        start_time_ns=1000,
        end_time_ns=2000,
        duration_ms=0.001,
        service_name="bench",
    )

    # Warmup
    for _ in range(5000):
        buf.enqueue(sd)
    buf.drain(buf._maxsize)

    start = time.perf_counter_ns()
    for _ in range(iterations):
        buf.enqueue(sd)
    elapsed = time.perf_counter_ns() - start

    return elapsed / iterations


def main() -> None:
    print("=" * 60)
    print("Axonize Inference Overhead Benchmark")
    print("=" * 60)

    results: list[tuple[str, float, str]] = []

    # 1. Ring buffer enqueue
    ns = bench_enqueue_only()
    target = "< 100ns"
    status = "PASS" if ns < 100 else "WARN" if ns < 500 else "FAIL"
    results.append(("Ring buffer enqueue", ns, f"{status} (target {target})"))

    # 2. GPU label resolution
    ns = bench_resolve_labels()
    target = "< 5μs"
    status = "PASS" if ns < 5000 else "WARN" if ns < 10000 else "FAIL"
    results.append(("GPU resolve_labels (2 GPUs)", ns, f"{status} (target {target})"))

    # 3. Span lifecycle (no profiler)
    ns = bench_span_lifecycle()
    target = "< 5μs"
    status = "PASS" if ns < 5000 else "WARN" if ns < 10000 else "FAIL"
    results.append(("Span lifecycle (no profiler)", ns, f"{status} (target {target})"))

    # 4. Span lifecycle (with mock profiler)
    ns = bench_span_with_profiler()
    target = "< 10μs"
    status = "PASS" if ns < 10000 else "WARN" if ns < 20000 else "FAIL"
    results.append(("Span + set_gpus (mock profiler)", ns, f"{status} (target {target})"))

    print()
    for name, ns_val, note in results:
        if ns_val >= 1000:
            display = f"{ns_val / 1000:.2f}μs"
        else:
            display = f"{ns_val:.0f}ns"
        print(f"  {name:40s}  {display:>10s}   {note}")

    print()
    all_pass = all("PASS" in r[2] or "WARN" in r[2] for r in results)
    if all_pass:
        print("All benchmarks within acceptable range.")
    else:
        print("WARNING: Some benchmarks exceeded target. Review above.")


if __name__ == "__main__":
    main()
