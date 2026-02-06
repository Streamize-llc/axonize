# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Axonize is an observability platform for AI inference workloads. It sits between infrastructure monitoring (Grafana/Prometheus) and LLM service tracing (Langfuse/LangSmith), focusing on inference-level GPU metrics and performance tracking.

## Commands

### Development
```bash
make dev              # Start ClickHouse + PostgreSQL containers only
make dev-all          # Start all services (DBs + server + dashboard)
make migrate          # Apply DB migrations (requires clickhouse-client + psql)
make clean            # Stop containers, remove volumes and build artifacts
make dev-dashboard    # Start dashboard dev server (hot-reload)
```

### Testing
```bash
make test             # Run all tests (SDK + server)
make test-sdk         # cd sdk-py && uv run pytest
make test-server      # cd server && go test ./...
make test-e2e         # E2E tests (requires make dev-all + make migrate first)
make test-load        # Load tests
```

Single test: `cd sdk-py && uv run pytest tests/test_specific.py::TestClass::test_name -v`

### Linting
```bash
make lint             # Run all linters
make lint-sdk         # ruff check + mypy (sdk-py)
make lint-server      # go vet (server)
```

### Python SDK setup
```bash
cd sdk-py && uv sync --python /opt/homebrew/bin/python3.13
```
Must use native Python — the uv-installed `python3.13` is a wasm32/emscripten build that cannot compile native extensions (ruff, grpcio).

## Architecture

### Data flow
```
SDK (Python) → gRPC (OTLP) → Server (Go) → ClickHouse (spans/metrics) + PostgreSQL (GPU registry)
                                          → REST API → Dashboard (React)
```

### SDK internal pipeline
Spans flow through a 4-stage pipeline, all designed for < 1μs inference thread overhead:
1. **Span** (`_span.py`, `_llm.py`) — Context manager collects attributes; parent-child linking via `contextvars` (`_context.py`)
2. **RingBuffer** (`_buffer.py`) — Lock-free `deque`-based buffer; span data written on `__exit__`
3. **BackgroundProcessor** (`_processor.py`) — Daemon thread drains buffer at `flush_interval_ms` intervals
4. **OTLPExporter** (`_exporter.py`) — Converts `SpanData` to OTel protobuf, ships via gRPC

GPU metrics are collected in a separate daemon thread by `GPUProfiler` and attached at span exit via `set_gpus()`.

### GPU backend architecture
- `_gpu_backend.py` — `GPUBackend` Protocol + `DiscoveredGPU` dataclass (the only shared dependency)
- `_gpu_nvml.py` — NVIDIA backend (pynvml, optional via `axonize[nvidia]`)
- `_gpu_apple.py` — Apple Silicon backend (IOKit via ctypes, auto-detected on macOS ARM64)
- `_gpu.py` — `GPUProfiler` (backend-agnostic), `MockGPUProfiler`, factory `create_gpu_profiler()`
- Factory auto-selects: NVIDIA → Apple Silicon → None (graceful degradation)
- `gpu.N.vendor` OTLP attribute carries vendor info SDK→Server

### Server structure
- `internal/ingest/handler.go` — OTLP gRPC handler: parses protobuf spans, extracts GPU attributes (`gpu.N.*`), batches to ClickHouse, upserts GPU registry to PostgreSQL
- `internal/store/` — ClickHouse (time-series spans + gpu_metrics) and PostgreSQL (GPU registry) stores
- `internal/api/` — REST endpoints for traces, GPUs, analytics
- `internal/config/` — YAML config with env var overrides (`AXONIZE_CONFIG` or default `config.yaml`)

### Databases
- **ClickHouse**: `spans`, `gpu_metrics` tables. Partitioned by day, TTL 7-90 days.
- **PostgreSQL**: 3-layer GPU identity model:
  1. `physical_gpus` — Immutable hardware (UUID, model, vendor, node)
  2. `compute_resources` — Logical unit (full GPU or MIG instance)
  3. `resource_contexts` — Runtime labels ("cuda:0", pod/container info)

This disambiguates MIG environments where every pod sees "cuda:0".

### Docker services
- `clickhouse` — ports 8123 (HTTP), 9000 (native)
- `postgres` — port 5432
- `axonize-server` — gRPC :4317, HTTP :8080
- `dashboard` — port 3000

## Key Conventions

- `ARCHITECTURE.md` is the single source of truth for schemas and data model decisions
- DB schema in `migrations/` must match ARCHITECTURE.md §6.2 (ClickHouse) and §6.3 (PostgreSQL)
- Conventional commits: `feat:`, `fix:`, `docs:`, `test:`, `refactor:`
- Python: `from __future__ import annotations` in all files; ruff line-length 100; mypy strict
- Python SDK pins `mypy>=1.10,<1.19` to avoid `librt` build failures on macOS
- Go: `log/slog` for structured logging; context-first function signatures; `fmt.Errorf("...: %w", err)` for error wrapping
- Go is not installed in the local dev environment — Go validation happens in CI
- Inference must never be affected by tracing failures — all exporter/GPU errors are swallowed with debug logging
