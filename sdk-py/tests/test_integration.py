"""Integration tests — full SDK lifecycle."""

import threading

import axonize
import axonize._sdk as sdk_mod
from axonize._types import SpanData, SpanKind, SpanStatus


def setup_function() -> None:
    sdk_mod._sdk_instance = None


def teardown_function() -> None:
    axonize.shutdown()


def test_full_lifecycle() -> None:
    """init → trace → nested spans → buffer has all spans."""
    collected: list[SpanData] = []

    axonize.init(endpoint="http://localhost:4317", service_name="integration-test")

    # Install a test handler to collect spans
    assert sdk_mod._sdk_instance is not None
    assert sdk_mod._sdk_instance._processor is not None
    sdk_mod._sdk_instance._processor._handler = collected.extend

    @axonize.trace(name="handle-request", kind=SpanKind.SERVER)
    def handle_request() -> str:
        with axonize.span("validate-input") as s:
            s.set_attribute("input_length", 100)

        with axonize.span("run-inference") as s:
            s.set_gpus(["cuda:0"])
            with axonize.span("tokenize"):
                pass
            with axonize.span("forward-pass") as inner:
                inner.set_attribute("model", "llama-3-8b")

        return "done"

    result = handle_request()
    assert result == "done"

    # Stop processor to trigger final drain
    axonize.shutdown()

    assert len(collected) == 5
    names = [s.name for s in collected]
    assert "handle-request" in names
    assert "validate-input" in names
    assert "run-inference" in names
    assert "tokenize" in names
    assert "forward-pass" in names

    # Verify parent-child relationships
    by_name = {s.name: s for s in collected}
    root = by_name["handle-request"]
    assert root.parent_span_id is None
    assert root.kind == SpanKind.SERVER

    validate = by_name["validate-input"]
    assert validate.parent_span_id == root.span_id
    assert validate.trace_id == root.trace_id

    inference = by_name["run-inference"]
    assert inference.parent_span_id == root.span_id
    # Without GPU profiler, attributions are empty
    assert inference.gpu_attributions == []

    tokenize = by_name["tokenize"]
    assert tokenize.parent_span_id == inference.span_id

    forward = by_name["forward-pass"]
    assert forward.parent_span_id == inference.span_id
    assert forward.attributes["model"] == "llama-3-8b"

    # All spans share the same trace ID
    trace_ids = {s.trace_id for s in collected}
    assert len(trace_ids) == 1


def test_deep_nesting() -> None:
    axonize.init(endpoint="http://localhost:4317", service_name="deep-test")
    assert sdk_mod._sdk_instance is not None
    buf = sdk_mod._sdk_instance._buffer
    assert buf is not None

    depth = 10
    def nest(level: int) -> None:
        if level == 0:
            return
        with axonize.span(f"level-{level}"):
            nest(level - 1)

    nest(depth)

    items = buf.drain(100)
    assert len(items) == depth

    # Verify chain: level-1's parent is level-2, etc.
    by_name = {s.name: s for s in items}
    for i in range(1, depth):
        child = by_name[f"level-{i}"]
        parent = by_name[f"level-{i + 1}"]
        assert child.parent_span_id == parent.span_id


def test_concurrent_traces() -> None:
    """Multiple threads creating independent traces."""
    collected: list[SpanData] = []
    lock = threading.Lock()

    axonize.init(endpoint="http://localhost:4317", service_name="concurrent-test")
    assert sdk_mod._sdk_instance is not None
    assert sdk_mod._sdk_instance._processor is not None

    def safe_handler(spans: list[SpanData]) -> None:
        with lock:
            collected.extend(spans)

    sdk_mod._sdk_instance._processor._handler = safe_handler

    def worker(worker_id: int) -> None:
        with axonize.span(f"worker-{worker_id}") as s:
            s.set_attribute("worker_id", worker_id)
            with axonize.span(f"task-{worker_id}"):
                pass

    threads = [threading.Thread(target=worker, args=(i,)) for i in range(5)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()

    axonize.shutdown()

    assert len(collected) == 10  # 5 workers × 2 spans each

    # Each worker pair should share a trace ID
    trace_ids = {s.trace_id for s in collected}
    assert len(trace_ids) == 5  # 5 independent traces


def test_error_in_nested_span() -> None:
    axonize.init(endpoint="http://localhost:4317", service_name="error-test")
    assert sdk_mod._sdk_instance is not None
    buf = sdk_mod._sdk_instance._buffer
    assert buf is not None

    try:
        with axonize.span("outer"):
            with axonize.span("inner"):
                raise ValueError("boom")
    except ValueError:
        pass

    items = buf.drain(10)
    assert len(items) == 2
    by_name = {s.name: s for s in items}

    assert by_name["inner"].status == SpanStatus.ERROR
    assert by_name["inner"].error_message == "boom"
    assert by_name["outer"].status == SpanStatus.ERROR
