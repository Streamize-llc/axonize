#!/usr/bin/env python3
"""Load test: measure ingest throughput (target: 10K spans/sec).

Sends spans directly via gRPC (bypassing the SDK's background processor)
to measure raw server ingest capacity.

Prerequisites:
    docker compose up -d   # Full stack running
    make migrate

Usage:
    python tests/load_test.py
    python tests/load_test.py --total 50000 --batch 500 --workers 4
"""

from __future__ import annotations

import argparse
import os
import sys
import time
import uuid
from concurrent.futures import ThreadPoolExecutor, as_completed

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "sdk-py", "src"))

from axonize._exporter import OTLPExporter, _build_export_request  # noqa: E402
from axonize._types import SpanData, SpanKind, SpanStatus  # noqa: E402

GRPC_ENDPOINT = os.environ.get("AXONIZE_GRPC", "localhost:4317")


def make_span(trace_id: str, seq: int) -> SpanData:
    """Create a realistic SpanData for load testing."""
    now = time.time_ns()
    duration_ns = 5_000_000  # 5ms
    return SpanData(
        span_id=uuid.uuid4().hex[:16],
        trace_id=trace_id,
        name=f"inference-{seq}",
        kind=SpanKind.INTERNAL,
        status=SpanStatus.OK,
        start_time_ns=now,
        end_time_ns=now + duration_ns,
        duration_ms=duration_ns / 1e6,
        service_name="load-test",
        environment="bench",
        attributes={
            "ai.model.name": "llama-3-70b",
            "ai.llm.tokens.input": 128,
            "ai.llm.tokens.output": 256,
            "batch_id": str(seq // 100),
        },
    )


def send_batch(exporter: OTLPExporter, batch_size: int, batch_num: int) -> tuple[int, float]:
    """Send a batch of spans and return (count, elapsed_seconds)."""
    trace_id = uuid.uuid4().hex
    spans = [make_span(trace_id, i) for i in range(batch_size)]

    start = time.monotonic()
    exporter.export(spans)
    elapsed = time.monotonic() - start

    return len(spans), elapsed


def run_load_test(total_spans: int, batch_size: int, workers: int) -> None:
    print(f"Load Test Configuration:")
    print(f"  Endpoint:    {GRPC_ENDPOINT}")
    print(f"  Total spans: {total_spans:,}")
    print(f"  Batch size:  {batch_size}")
    print(f"  Workers:     {workers}")
    print()

    exporter = OTLPExporter(
        GRPC_ENDPOINT,
        "load-test",
        "bench",
        timeout_s=30.0,
    )

    num_batches = total_spans // batch_size
    sent = 0
    errors = 0

    start = time.monotonic()

    with ThreadPoolExecutor(max_workers=workers) as pool:
        futures = []
        for i in range(num_batches):
            f = pool.submit(send_batch, exporter, batch_size, i)
            futures.append(f)

        for f in as_completed(futures):
            try:
                count, elapsed = f.result()
                sent += count
            except Exception as e:
                errors += 1
                print(f"  Batch error: {e}")

    total_elapsed = time.monotonic() - start
    exporter.shutdown()

    # Results
    rate = sent / total_elapsed if total_elapsed > 0 else 0

    print(f"\nResults:")
    print(f"  Sent:     {sent:,} spans")
    print(f"  Errors:   {errors}")
    print(f"  Duration: {total_elapsed:.2f}s")
    print(f"  Rate:     {rate:,.0f} spans/sec")
    print()

    target = 10_000
    if rate >= target:
        print(f"  [PASS] Achieved {rate:,.0f} spans/sec (target: {target:,})")
    else:
        print(f"  [FAIL] Only {rate:,.0f} spans/sec (target: {target:,})")
        sys.exit(1)


def main() -> None:
    parser = argparse.ArgumentParser(description="Axonize ingest load test")
    parser.add_argument("--total", type=int, default=50_000, help="Total spans to send")
    parser.add_argument("--batch", type=int, default=500, help="Spans per batch")
    parser.add_argument("--workers", type=int, default=4, help="Concurrent workers")
    args = parser.parse_args()

    run_load_test(args.total, args.batch, args.workers)


if __name__ == "__main__":
    main()
