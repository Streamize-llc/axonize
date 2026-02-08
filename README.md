# Axonize

**Observability for AI inference workloads.** Track every inference call with GPU-level metrics, latency breakdown, and cost tracking.

Axonize sits between infrastructure monitoring (Grafana/Prometheus) and LLM service tracing (Langfuse/LangSmith), giving you inference-level visibility into how your models use GPU resources.

## Key Features

- **Low-overhead tracing** — Lock-free ring buffer designed for <1μs overhead per span
- **Multi-vendor GPU support** — Auto-detects NVIDIA (via pynvml) and Apple Silicon (via IOKit)
- **3-layer GPU identity** — Physical GPU → Compute Resource → Runtime Context (MIG-aware)
- **LLM Metrics** — TTFT, tokens/sec, and token-level streaming tracking out of the box
- **OpenTelemetry Compatible** — OTLP export with `ai.*`, `gpu.*`, `cost.*` semantic conventions
- **Full-stack Dashboard** — Trace timeline (Gantt), GPU monitoring, latency analytics
- **Multi-tenant ready** — API key authentication and usage tracking for cloud deployments

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
git clone https://github.com/Streamize-llc/axonize.git
cd axonize

# Start ClickHouse + PostgreSQL + Server + Dashboard
docker compose up -d

# Apply database migrations (waits for databases to be ready)
make migrate
```

The dashboard is available at `http://localhost:3000`.

### 2. Set up authentication

The server generates a default API key on first startup. Get it from the logs:

```bash
docker compose logs axonize-server | grep "API key"
# Output: Generated API key: axon_...
```

Or set your own key in `.env`:

```bash
AXONIZE_API_KEY=your-secret-key
```

### 3. Install the SDK

```bash
# Basic installation (includes Apple Silicon GPU support)
pip install axonize

# For NVIDIA GPUs (requires CUDA drivers)
pip install axonize[nvidia]

# All optional dependencies
pip install axonize[all]
```

### 4. Instrument your code

```python
import os
import axonize

axonize.init(
    endpoint="localhost:4317",
    service_name="my-inference-service",
    api_key=os.getenv("AXONIZE_API_KEY"),  # Required for authentication
)

# Basic span
with axonize.span("image-generation") as s:
    s.set_attribute("ai.model.name", "stable-diffusion-xl")
    s.set_gpus(["cuda:0"])  # or ["mps:0"] for Apple Silicon
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

### 5. View traces

Open `http://localhost:3000` to see your traces, GPU metrics, and performance analytics.

## SDK API

### Initialization

```python
axonize.init(
    endpoint="localhost:4317",     # gRPC endpoint
    service_name="my-service",     # Service identifier
    api_key="your-api-key",        # Authentication (required if server has AXONIZE_API_KEY set)
    environment="production",      # Environment tag
    gpu_profiling=True,            # Enable GPU profiling (auto-detects vendor)
    batch_size=512,                # Spans per export batch
    flush_interval_ms=5000,        # Max time between flushes
    sampling_rate=1.0,             # Fraction of spans to keep (0.0-1.0)
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
1. **Auto-detects GPU backend**: NVIDIA (pynvml) or Apple Silicon (IOKit)
2. Discovers available GPUs (including NVIDIA MIG instances)
3. Collects metrics every 100ms in a background thread (utilization, memory, power, temperature)
4. Attaches GPU attribution when you call `span.set_gpus()`

**Device labels**:
- NVIDIA: `cuda:0`, `cuda:1`, etc.
- Apple Silicon: `mps:0`

**Installation**:
- Apple Silicon support is built-in (no extra dependencies)
- NVIDIA requires: `pip install axonize[nvidia]` (installs pynvml)

```python
axonize.init(endpoint="...", service_name="...", gpu_profiling=True)

with axonize.span("inference") as s:
    # NVIDIA
    s.set_gpus(["cuda:0"])

    # Apple Silicon
    # s.set_gpus(["mps:0"])

    # GPU metrics (utilization, memory, power, temp) are
    # automatically attached to this span with vendor-specific values
```

## Server Endpoints

**Authentication**: All `/api/v1/*` endpoints require Bearer token authentication (except health checks):

```bash
curl -H "Authorization: Bearer <your-api-key>" \
  http://localhost:8080/api/v1/traces
```

### Public Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /healthz` | Health check (no auth) |
| `GET /readyz` | Readiness check - DB connectivity (no auth) |

### Query API (requires API key)

| Endpoint | Description |
|----------|-------------|
| `GET /api/v1/traces` | List traces (filterable, paginated) |
| `GET /api/v1/traces/{id}` | Trace detail with span tree |
| `GET /api/v1/gpus` | GPU registry list |
| `GET /api/v1/gpus/{uuid}` | GPU detail |
| `GET /api/v1/gpus/{uuid}/metrics` | GPU metric time series |
| `GET /api/v1/analytics/overview` | Dashboard analytics (throughput, latency, errors) |

### Admin API (requires admin key, multi-tenant mode only)

Available when `AXONIZE_AUTH_MODE=multi_tenant`. Requires `Authorization: Bearer <AXONIZE_ADMIN_KEY>`.

| Endpoint | Description |
|----------|-------------|
| `POST /api/v1/admin/tenants` | Create new tenant |
| `GET /api/v1/admin/tenants` | List all tenants |
| `POST /api/v1/admin/tenants/{id}/keys` | Create API key for tenant |
| `DELETE /api/v1/admin/tenants/{id}/keys/{prefix}` | Revoke API key |
| `GET /api/v1/admin/tenants/{id}/usage` | Get tenant usage (spans + GPU seconds) |

See [`docs/server-deployment.md`](docs/server-deployment.md) for multi-tenant setup and billing integration.

## Configuration

All server configuration is via environment variables. See [`.env.example`](.env.example) for full reference. Key variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `AXONIZE_API_KEY` | (generated) | Static API key for single-tenant mode |
| `AXONIZE_AUTH_MODE` | `static` | `static` (single tenant) or `multi_tenant` |
| `AXONIZE_ADMIN_KEY` | (none) | Admin API key (multi-tenant mode only) |
| `CLICKHOUSE_HOST` | `clickhouse` | ClickHouse host |
| `POSTGRES_HOST` | `postgres` | PostgreSQL host |
| `GRPC_PORT` | `4317` | gRPC ingest port |
| `HTTP_PORT` | `8080` | HTTP API port |

**Data retention** (ClickHouse TTL):
- Spans: 30 days
- Traces: 90 days
- GPU metrics: 7 days

## Development

```bash
# Start dev databases (ClickHouse + PostgreSQL only)
make dev

# Run SDK tests
make test-sdk

# Run all linters
make lint

# Start dashboard dev server (hot-reload on port 3000)
make dev-dashboard

# Run E2E tests (requires full stack)
make dev-all && make migrate && make test-e2e
```

## Examples

**Prerequisites**: All examples require the server to be running (`docker compose up -d && make migrate`).

See the [`examples/`](examples/) directory for integration guides:

- [`quickstart.py`](examples/quickstart.py) — Minimal setup
- [`vllm_integration.py`](examples/vllm_integration.py) — vLLM with streaming tokens
- [`ollama_integration.py`](examples/ollama_integration.py) — Ollama chat with TTFT tracking
- [`diffusers_integration.py`](examples/diffusers_integration.py) — HuggingFace Diffusers pipeline
- [`custom_model.py`](examples/custom_model.py) — General-purpose integration pattern

## Project Structure

```
axonize/
├── sdk-py/                    Python SDK (pip install axonize)
├── server/                    Go ingest + query server
│   ├── internal/ingest/       OTLP gRPC handler
│   ├── internal/api/          REST API endpoints
│   ├── internal/store/        ClickHouse + PostgreSQL stores
│   └── internal/tenant/       Multi-tenant auth + usage tracking
├── dashboard/                 React + Vite dashboard
├── migrations/                Database schemas
│   ├── clickhouse/            Spans, traces, gpu_metrics tables
│   └── postgres/              GPU registry + tenant management
├── examples/                  Integration examples
├── tests/                     E2E + load tests
├── docs/                      Documentation
├── .env.example               Configuration reference
└── config.yaml                Server config (overridden by env vars)
```

## License

Apache-2.0
