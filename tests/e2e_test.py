#!/usr/bin/env python3
"""E2E test: SDK → gRPC → Server → ClickHouse → REST API.

Prerequisites:
    docker compose up -d   # ClickHouse + PostgreSQL + axonize-server
    make migrate           # Apply DB migrations

Usage:
    python tests/e2e_test.py
    # or with custom endpoints:
    AXONIZE_GRPC=localhost:4317 AXONIZE_HTTP=http://localhost:8080 python tests/e2e_test.py
"""

from __future__ import annotations

import json
import os
import sys
import time
import urllib.request
import urllib.error

# Add sdk-py to path so we can import axonize without installing
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "sdk-py", "src"))

import axonize  # noqa: E402

GRPC_ENDPOINT = os.environ.get("AXONIZE_GRPC", "localhost:4317")
HTTP_ENDPOINT = os.environ.get("AXONIZE_HTTP", "http://localhost:8080")

PASS = 0
FAIL = 0


def check(name: str, condition: bool, detail: str = "") -> None:
    global PASS, FAIL
    if condition:
        PASS += 1
        print(f"  [PASS] {name}")
    else:
        FAIL += 1
        msg = f"  [FAIL] {name}"
        if detail:
            msg += f" — {detail}"
        print(msg)


def api_get(path: str) -> dict | None:
    """GET request to the HTTP API."""
    url = f"{HTTP_ENDPOINT}{path}"
    try:
        req = urllib.request.Request(url)
        with urllib.request.urlopen(req, timeout=5) as resp:
            return json.loads(resp.read())
    except urllib.error.URLError as e:
        print(f"  [ERROR] GET {url}: {e}")
        return None


def test_health() -> None:
    print("\n=== Health Checks ===")
    data = api_get("/healthz")
    check("GET /healthz returns ok", data is not None and data.get("status") == "ok")

    data = api_get("/readyz")
    check("GET /readyz returns ready", data is not None and data.get("status") == "ready")


def test_e2e_pipeline() -> None:
    print("\n=== E2E Pipeline: SDK → Server → ClickHouse → API ===")

    # 1. Initialize SDK
    service_name = f"e2e-test-{int(time.time())}"
    axonize.init(
        endpoint=GRPC_ENDPOINT,
        service_name=service_name,
        environment="e2e-test",
        batch_size=10,
        flush_interval_ms=500,
    )
    print(f"  SDK initialized (service={service_name})")

    # 2. Create spans
    trace_ids = set()
    with axonize.span("root-operation") as root:
        root.set_attribute("ai.model.name", "test-model")
        root.set_attribute("batch_size", 32)
        trace_id = root.trace_id
        trace_ids.add(trace_id)

        with axonize.span("child-operation") as child:
            child.set_attribute("step", "preprocess")
            child.set_gpus(["cuda:0"])
            time.sleep(0.01)

        with axonize.span("another-child") as child2:
            child2.set_attribute("step", "inference")
            time.sleep(0.01)

    print(f"  Created 3 spans (trace_id={trace_id[:16]}...)")

    # 3. Shutdown to flush all spans
    axonize.shutdown()
    print("  SDK shutdown (flushed)")

    # 4. Wait for server to flush its batch buffer
    time.sleep(3)

    # 5. Query traces
    data = api_get(f"/api/v1/traces?service_name={service_name}")
    if data is None:
        check("Traces API reachable", False, "Could not reach API")
        return

    traces = data.get("traces", [])
    check("Trace found in list", len(traces) >= 1, f"got {len(traces)} traces")

    if traces:
        t = traces[0]
        check("Trace has correct service_name", t.get("service_name") == service_name)
        check("Trace has span_count >= 3", t.get("span_count", 0) >= 3,
              f"got {t.get('span_count')}")
        check("Trace has error_count == 0", t.get("error_count", -1) == 0)

    # 6. Query trace detail
    data = api_get(f"/api/v1/traces/{trace_id}")
    if data is None:
        check("Trace detail API reachable", False, "Could not reach API")
        return

    check("Trace detail found", data.get("trace_id") == trace_id)
    spans = data.get("spans", [])
    check("Root span(s) present", len(spans) >= 1, f"got {len(spans)} root spans")

    # Check nested structure
    if spans:
        root_span = spans[0]
        children = root_span.get("children", [])
        check("Root has children", len(children) >= 2,
              f"got {len(children)} children")


def test_error_span() -> None:
    print("\n=== Error Span Handling ===")

    service_name = f"e2e-error-{int(time.time())}"
    axonize.init(
        endpoint=GRPC_ENDPOINT,
        service_name=service_name,
        environment="e2e-test",
        batch_size=10,
        flush_interval_ms=500,
    )

    trace_id = None
    try:
        with axonize.span("failing-operation") as s:
            trace_id = s.trace_id
            raise ValueError("intentional error")
    except ValueError:
        pass

    axonize.shutdown()
    time.sleep(3)

    if trace_id:
        data = api_get(f"/api/v1/traces?service_name={service_name}")
        if data and data.get("traces"):
            t = data["traces"][0]
            check("Error span recorded", t.get("error_count", 0) >= 1,
                  f"error_count={t.get('error_count')}")


def test_gpu_api() -> None:
    print("\n=== GPU API Endpoints ===")

    # List GPUs (may be empty without real GPUs, but endpoint should work)
    data = api_get("/api/v1/gpus")
    check("GET /api/v1/gpus returns response", data is not None)
    if data is not None:
        check("Response has gpus array", isinstance(data.get("gpus"), list))

    # Get a non-existent GPU (should return 404)
    url = f"{HTTP_ENDPOINT}/api/v1/gpus/GPU-nonexistent"
    try:
        req = urllib.request.Request(url)
        urllib.request.urlopen(req, timeout=5)
        check("GET /api/v1/gpus/:uuid returns 404 for missing", False)
    except urllib.error.HTTPError as e:
        check("GET /api/v1/gpus/:uuid returns 404 for missing", e.code == 404)
    except urllib.error.URLError:
        check("GPU detail API reachable", False, "Could not reach API")

    # GPU metrics (should return empty array for non-existent GPU)
    data = api_get("/api/v1/gpus/GPU-nonexistent/metrics")
    check("GET /api/v1/gpus/:uuid/metrics returns response", data is not None)
    if data is not None:
        check("Response has metrics array", isinstance(data.get("metrics"), list))


def main() -> None:
    print(f"Axonize E2E Test")
    print(f"  gRPC: {GRPC_ENDPOINT}")
    print(f"  HTTP: {HTTP_ENDPOINT}")

    test_health()
    test_e2e_pipeline()
    test_error_span()
    test_gpu_api()

    print(f"\n{'='*40}")
    print(f"Results: {PASS} passed, {FAIL} failed")

    if FAIL > 0:
        sys.exit(1)
    print("All E2E tests passed!")


if __name__ == "__main__":
    main()
