# SDK API Reference

## Module: `axonize`

### `axonize.init(**kwargs) -> None`

Initialize the Axonize SDK. Must be called before creating any spans.

```python
import os
import axonize

axonize.init(
    endpoint="localhost:4317",
    service_name="my-service",
    api_key=os.getenv("AXONIZE_API_KEY"),  # Required if server has authentication enabled
    environment="production",
    gpu_profiling=True,
    batch_size=512,
    flush_interval_ms=5000,
    buffer_size=8192,
    sampling_rate=1.0,
)
```

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `endpoint` | `str` | ✅ | - | gRPC server address (`host:port`) |
| `service_name` | `str` | ✅ | - | Service identifier |
| `api_key` | `str \| None` | ⚠️ | `None` | Authentication key. Required if server has `AXONIZE_API_KEY` set |
| `environment` | `str` | ❌ | `"development"` | Environment tag (e.g., "production", "staging") |
| `gpu_profiling` | `bool` | ❌ | `False` | Enable GPU profiling (auto-detects NVIDIA or Apple Silicon) |
| `batch_size` | `int` | ❌ | `512` | Number of spans per export batch |
| `flush_interval_ms` | `int` | ❌ | `5000` | Maximum milliseconds between flushes |
| `buffer_size` | `int` | ❌ | `8192` | Ring buffer capacity (max spans before blocking) |
| `sampling_rate` | `float` | ❌ | `1.0` | Fraction of spans to keep (0.0 = drop all, 1.0 = keep all) |

### `axonize.shutdown() -> None`

Shut down the SDK, flushing all remaining spans. Automatically registered with `atexit`.

### `axonize.span(name, *, kind=SpanKind.INTERNAL) -> Span`

Create a general-purpose span context manager.

```python
import axonize

with axonize.span("operation") as s:
    s.set_attribute("key", "value")
    s.set_gpus(["cuda:0"])  # Optional: attach GPU metrics
```

### `axonize.llm_span(name, *, model=None, model_version=None, inference_type="llm", kind=SpanKind.SERVER) -> LLMSpan`

Create an LLM-specialized span with token tracking.

```python
import axonize

with axonize.llm_span("generate", model="llama-3-70b") as s:
    s.set_tokens_input(128)
    s.set_gpus(["cuda:0", "cuda:1"])  # Multi-GPU inference
    for token in stream:
        s.record_token()
    # Automatically calculates TTFT, tokens/sec, token count
```

### `@axonize.trace`

Decorator to wrap a function in a span.

```python
import axonize

@axonize.trace
def my_function():
    # Span name defaults to "my_function"
    result = do_inference()
    return result

@axonize.trace(name="custom-name")
def another_function(x, y):
    # Span name is "custom-name"
    return x + y
```

---

## Class: `Span`

### `Span.set_attribute(key: str, value: str | int | float | bool) -> None`

Attach a key-value attribute to the span.

### `Span.set_gpus(labels: list[str]) -> None`

Set GPU device labels (e.g., `["cuda:0", "cuda:1"]` for NVIDIA or `["mps:0"]` for Apple Silicon). If GPU profiling is enabled, automatically resolves to full GPU attribution with hardware identity and live metrics.

### `Span.set_status(status: SpanStatus, message: str | None = None) -> None`

Explicitly set the span status.

### Properties

| Property | Type | Description |
|----------|------|-------------|
| `span_id` | `str` | Unique span identifier (16 hex chars) |
| `trace_id` | `str` | Trace identifier (32 hex chars) |
| `parent_span_id` | `str | None` | Parent span ID (auto-detected) |
| `name` | `str` | Span name |
| `kind` | `SpanKind` | Span type |

---

## Class: `LLMSpan` (extends `Span`)

### `LLMSpan.record_token() -> None`

Record a single output token. Call once per generated token during streaming. Automatically tracks TTFT and token count.

### `LLMSpan.set_tokens_input(count: int) -> None`

Set the number of input (prompt) tokens.

### `LLMSpan.set_tokens_output(count: int) -> None`

Manually set output token count (alternative to calling `record_token()` repeatedly).

### `LLMSpan.set_model(name: str, version: str | None = None) -> None`

Set model name and optional version after creation.

### Auto-calculated Attributes

On span exit, `LLMSpan` automatically computes and sets:

| Attribute | Description |
|-----------|-------------|
| `ai.llm.ttft_ms` | Time from span start to first `record_token()` call |
| `ai.llm.tokens_per_second` | Output tokens / generation duration |
| `ai.llm.tokens.output` | Total output token count |
| `ai.llm.tokens.input` | Input token count (if set) |

---

## Enums

### `SpanKind`

| Value | Description |
|-------|-------------|
| `INTERNAL` | Internal operation (default) |
| `CLIENT` | Outgoing call |
| `SERVER` | Incoming request |

### `SpanStatus`

| Value | Description |
|-------|-------------|
| `UNSET` | Not explicitly set |
| `OK` | Successful |
| `ERROR` | Failed |

---

## Dataclasses

### `SpanData` (frozen)

Immutable snapshot of a completed span. Fields: `span_id`, `trace_id`, `name`, `kind`, `status`, `start_time_ns`, `end_time_ns`, `duration_ms`, `service_name`, `attributes`, `parent_span_id`, `gpu_attributions`, `error_message`, `environment`.

### `GPUAttribution` (frozen)

GPU metrics snapshot attached to a span. Fields: `resource_uuid`, `physical_gpu_uuid`, `gpu_model`, `vendor`, `node_id`, `resource_type`, `user_label`, `memory_used_gb`, `memory_total_gb`, `utilization`, `temperature_celsius`, `power_watts`, `clock_mhz`.

---

## Attribute Conventions

Axonize follows OpenTelemetry semantic conventions with these prefixes:

| Prefix | Purpose | Examples |
|--------|---------|----------|
| `ai.model.*` | Model identity | `ai.model.name`, `ai.model.version` |
| `ai.inference.*` | Inference type | `ai.inference.type` |
| `ai.llm.*` | LLM metrics | `ai.llm.tokens.input`, `ai.llm.ttft_ms` |
| `ai.diffusion.*` | Diffusion params | `ai.diffusion.steps`, `ai.diffusion.cfg_scale` |
| `gpu.N.*` | GPU attribution | `gpu.0.resource_uuid`, `gpu.0.vendor`, `gpu.0.utilization` |
| `cost.*` | Cost tracking (user-provided) | `cost.usd` |
