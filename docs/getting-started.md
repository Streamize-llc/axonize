# Getting Started

This guide walks you through setting up Axonize and sending your first traces.

## Prerequisites

- Docker and Docker Compose
- Python 3.10+

## 1. Start the Server

```bash
git clone https://github.com/your-org/axonize.git
cd axonize

# Start all services (ClickHouse, PostgreSQL, Server, Dashboard)
docker compose up -d

# Apply database migrations
make migrate
```

Verify the server is running:

```bash
curl http://localhost:8080/healthz
# {"status":"ok"}
```

The dashboard is at `http://localhost:3000`.

## 2. Install the SDK

```bash
# Basic install (Apple Silicon GPU support included)
pip install axonize

# With NVIDIA GPU support
pip install axonize[nvidia]

# All optional backends
pip install axonize[all]
```

Or for development:

```bash
cd sdk-py
pip install -e ".[dev]"
```

## 3. Initialize and Trace

```python
import axonize

# Connect to the Axonize server
axonize.init(
    endpoint="localhost:4317",     # gRPC ingest endpoint
    service_name="my-service",     # Identifies your service
    environment="development",     # Environment tag
)

# Create a traced operation
with axonize.span("my-operation") as s:
    s.set_attribute("batch_size", 32)
    # Your inference code here
    result = do_inference()

# Flush remaining spans on shutdown
axonize.shutdown()
```

## 4. View Your Traces

Open `http://localhost:3000/traces` to see your traced operations.

Click a trace to see the span timeline (Gantt chart), hierarchical tree, and individual span details.

## 5. Add GPU Profiling

Enable automatic GPU metric collection:

```python
axonize.init(
    endpoint="localhost:4317",
    service_name="gpu-service",
    gpu_profiling=True,  # Auto-detects NVIDIA or Apple Silicon
)

# NVIDIA GPUs
with axonize.span("gpu-inference") as s:
    s.set_gpus(["cuda:0"])  # Attach GPU metrics to this span
    result = model(input_tensor)

# Apple Silicon (M1/M2/M3/M4)
with axonize.span("mps-inference") as s:
    s.set_gpus(["mps:0"])  # Apple Silicon uses mps:N labels
    result = model(input_tensor)
```

GPU metrics (utilization, memory, power, temperature) are automatically collected at 100ms intervals and attached to spans.

**Supported GPU backends:**

| Platform | Backend | Labels | Install |
|----------|---------|--------|---------|
| NVIDIA GPUs | pynvml | `cuda:0`, `cuda:1`, ... | `pip install axonize[nvidia]` |
| Apple Silicon | IOKit | `mps:0` | `pip install axonize` (built-in) |

Apple Silicon notes:
- Unified memory is reported as total GPU memory
- Some metrics (temperature, clock) report as 0 (unavailable via IOKit)
- GPU utilization is derived from IOKit performance state residency

## 6. Track LLM Metrics

Use `llm_span` for language model workloads:

```python
with axonize.llm_span("generate", model="llama-3-70b") as s:
    s.set_tokens_input(128)

    for token in model.generate_stream(prompt):
        s.record_token()  # Track each output token
        yield token

    # Automatically calculates:
    #   - TTFT (Time To First Token)
    #   - Tokens per second
    #   - Total output tokens
```

## Configuration Reference

| Parameter | Default | Description |
|-----------|---------|-------------|
| `endpoint` | (required) | gRPC server address (host:port) |
| `service_name` | (required) | Service identifier |
| `environment` | `"development"` | Environment tag |
| `gpu_profiling` | `False` | Enable pynvml GPU profiling |
| `batch_size` | `512` | Spans per export batch |
| `flush_interval_ms` | `5000` | Max ms between flushes |
| `buffer_size` | `8192` | Ring buffer capacity |
| `sampling_rate` | `1.0` | Fraction of spans to keep (0.0-1.0) |

## Next Steps

- See [SDK API Reference](sdk-reference.md) for the full API
- Check [Examples](../examples/) for framework integrations
- Read [Server Deployment](server-deployment.md) for production setup
