# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Axonize is an observability platform for AI inference workloads. It sits between infrastructure monitoring (Grafana/Prometheus) and LLM service tracing (Langfuse/LangSmith), focusing on inference-level GPU metrics and performance tracking.

## Commands

### Development
```bash
make dev          # Start ClickHouse + PostgreSQL containers
make migrate      # Apply DB migrations (requires clickhouse-client + psql)
make clean        # Stop containers, remove volumes and build artifacts
```

### Testing
```bash
make test         # Run all tests (SDK + server)
make test-sdk     # cd sdk-py && uv run pytest
make test-server  # cd server && go test ./...
```

Single test: `cd sdk-py && uv run pytest tests/test_specific.py::test_name -v`

### Linting
```bash
make lint         # Run all linters
make lint-sdk     # ruff check + mypy (sdk-py)
make lint-server  # go vet (server)
```

### Python SDK setup
```bash
cd sdk-py && uv sync --python /opt/homebrew/bin/python3.13
```
Must use native Python — the uv-installed `python3.13` is a wasm32/emscripten build that cannot compile native extensions (ruff, grpcio).

## Architecture

### Monorepo layout
- `sdk-py/` — Python SDK (hatchling build, uv package manager)
- `server/` — Go server (go 1.23, yaml.v3 config)
- `dashboard/` — React frontend (placeholder, M4)
- `migrations/` — Raw SQL for ClickHouse and PostgreSQL

### Data flow
SDK (Python) → gRPC (OTLP) → Server (Go) → ClickHouse (spans/metrics) + PostgreSQL (GPU registry)

### Databases
- **ClickHouse**: Time-series data — `spans`, `traces`, `gpu_metrics` tables. Partitioned by day, TTL 7-90 days.
- **PostgreSQL**: GPU registry — `physical_gpus`, `compute_resources`, `resource_contexts` tables with a 3-layer identity model.

### 3-Layer GPU Identity Model
1. **Physical** (`physical_gpus`): Immutable hardware — GPU UUID, model, node location
2. **Resource** (`compute_resources`): Logical compute unit (PK) — full GPU or MIG instance
3. **Context** (`resource_contexts`): Runtime mapping — user labels like "cuda:0", pod/container info

This design allows MIG environments where every pod sees "cuda:0" to be disambiguated at every layer.

### Server config loading
`server/internal/config/config.go` loads config from YAML file (default `config.yaml`, override with `AXONIZE_CONFIG` env var), then applies environment variable overrides. All env vars match `.env.example`.

### SDK design constraints
- Inference thread overhead target: < 1μs (lock-free ring buffer, async GPU profiling)
- OpenTelemetry compatible — uses OTel semantic conventions with `ai.*`, `gpu.*`, `cost.*` attribute prefixes
- Graceful degradation — inference must not be affected by tracing failures

## Key Conventions

- `ARCHITECTURE.md` is the single source of truth for schemas and data model decisions
- DB schema in `migrations/` must match ARCHITECTURE.md §6.2 (ClickHouse) and §6.3 (PostgreSQL)
- Python SDK pins `mypy>=1.10,<1.19` to avoid `librt` build failures on macOS
- Go is not installed in the local dev environment — Go validation happens in CI
