# Axonize

**Observability for AI inference workloads.** Track every inference call with GPU-level metrics, latency breakdown, and cost tracking.

Axonize sits between infrastructure monitoring (Grafana/Prometheus) and LLM service tracing (Langfuse/LangSmith), giving you inference-level visibility into how your models use GPU resources.

## Key Features

- **Sub-microsecond overhead** — Lock-free ring buffer keeps tracing off the critical path
- **GPU Attribution** — Automatic pynvml profiling with 3-layer identity (Physical GPU -> Compute Resource -> Runtime Context)
- **LLM Metrics** — TTFT, tokens/sec, and token-level streaming tracking out of the box
- **MIG Support** — Disambiguate "cuda:0" across pods and MIG partitions
- **OpenTelemetry Compatible** — OTLP export with `ai.*`, `gpu.*`, `cost.*` semantic conventions
- **Full-stack Dashboard** — Trace timeline (Gantt), GPU monitoring, latency analytics

## Architecture

```
Python SDK (axonize)          Go Server              Databases
=====================    ==================    ==================
 span() / llm_span()         gRPC Ingest
        |                        |
   Ring Buffer               Batch Writer ----> ClickHouse
        |                        |               (spans, gpu_metrics)
 Background Processor        GPU Registry ----> PostgreSQL
        |                        |               (physical_gpus,
   OTLP gRPC Export          REST API             compute_resources)
                                 |
                             Dashboard (React)
```

## Quick Start

### 1. Start the server

```bash
git clone https://github.com/your-org/axonize.git
cd axonize

# Start ClickHouse + PostgreSQL + Server + Dashboard
docker compose up -d

# Apply database migrations
make migrate
```

The dashboard is available at `http://localhost:3000`.

### 2. Install the SDK

```bash
pip install axonize
```

### 3. Instrument your code

```python
import axonize

axonize.init(
    endpoint="localhost:4317",
    service_name="my-inference-service",
)

# Basic span
with axonize.span("image-generation") as s:
    s.set_attribute("ai.model.name", "stable-diffusion-xl")
    s.set_gpus(["cuda:0"])
    result = model.generate(prompt)

# LLM span with streaming token tracking
with axonize.llm_span("generate", model="llama-3-70b") as s:
    s.set_tokens_input(len(prompt_tokens))
    for token in model.generate_stream(prompt):
        s.record_token()
        yield token
    # TTFT and tokens/sec are calculated automatically

axonize.shutdown()
```

### 4. View traces

Open `http://localhost:3000` to see your traces, GPU metrics, and performance analytics.

## SDK API

### Initialization

```python
axonize.init(
    endpoint="localhost:4317",     # gRPC endpoint
    service_name="my-service",     # Service identifier
    environment="production",      # Environment tag
    gpu_profiling=True,            # Enable pynvml GPU profiling
    batch_size=512,                # Spans per export batch
    flush_interval_ms=5000,        # Max time between flushes
)
```

### Spans

```python
# Context manager
with axonize.span("operation-name") as s:
    s.set_attribute("key", "value")
    s.set_gpus(["cuda:0", "cuda:1"])
    s.set_status(axonize.SpanStatus.OK)

# Decorator
@axonize.trace
def my_function():
    pass

@axonize.trace(name="custom-name")
def another_function():
    pass
```

### LLM Spans

```python
with axonize.llm_span("generate", model="llama-3-70b") as s:
    s.set_tokens_input(128)        # Prompt token count
    s.set_model("llama-3", "70b")  # Model name + version

    for token in stream:
        s.record_token()           # Tracks each output token

    # On exit, automatically calculates:
    #   ai.llm.ttft_ms           — Time to first token
    #   ai.llm.tokens_per_second — Generation throughput
    #   ai.llm.tokens.output     — Total output tokens
```

### GPU Profiling

When `gpu_profiling=True`, the SDK automatically:
1. Discovers GPUs via pynvml (including MIG instances)
2. Collects metrics every 100ms in a background thread
3. Attaches GPU attribution when you call `span.set_gpus()`

```python
axonize.init(endpoint="...", service_name="...", gpu_profiling=True)

with axonize.span("inference") as s:
    s.set_gpus(["cuda:0"])
    # GPU metrics (utilization, memory, power, temp) are
    # automatically attached to this span
```

## Server Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /healthz` | Health check |
| `GET /readyz` | Readiness check (DB connectivity) |
| `GET /api/v1/traces` | List traces (filterable, paginated) |
| `GET /api/v1/traces/{id}` | Trace detail with span tree |
| `GET /api/v1/gpus` | GPU registry list |
| `GET /api/v1/gpus/{uuid}` | GPU detail |
| `GET /api/v1/gpus/{uuid}/metrics` | GPU metric time series |
| `GET /api/v1/analytics/overview` | Dashboard analytics |
| `POST /api/v1/admin/tenants` | Create tenant (admin key required) |
| `GET /api/v1/admin/tenants` | List tenants (admin key required) |
| `POST /api/v1/admin/tenants/{id}/keys` | Create API key for tenant |
| `DELETE /api/v1/admin/tenants/{id}/keys/{prefix}` | Revoke API key (admin key required) |
| `GET /api/v1/admin/tenants/{id}/usage` | Tenant usage (spans + GPU seconds) |

## Development

```bash
# Start dev databases
make dev

# Run SDK tests
make test-sdk

# Run all linters
make lint

# Start dashboard dev server
make dev-dashboard

# Run E2E tests (requires full stack)
make dev-all && make migrate && make test-e2e
```

## Examples

See the [`examples/`](examples/) directory for integration guides:

- [`quickstart.py`](examples/quickstart.py) — Minimal setup
- [`vllm_integration.py`](examples/vllm_integration.py) — vLLM with streaming tokens
- [`ollama_integration.py`](examples/ollama_integration.py) — Ollama chat with TTFT tracking
- [`diffusers_integration.py`](examples/diffusers_integration.py) — HuggingFace Diffusers pipeline
- [`custom_model.py`](examples/custom_model.py) — General-purpose integration pattern

## Project Structure

```
axonize/
├── sdk-py/          Python SDK (pip install axonize)
├── server/          Go ingest + query server
├── dashboard/       React dashboard
├── migrations/      ClickHouse + PostgreSQL schemas
├── examples/        Integration examples
├── tests/           E2E tests
└── docs/            Documentation
```

## License

Apache-2.0
