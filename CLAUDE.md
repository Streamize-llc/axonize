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
- `internal/server.go` — Orchestrates gRPC + HTTP listeners, wires auth interceptors based on `auth_mode`
- `internal/ingest/handler.go` — OTLP gRPC handler: parses protobuf spans, extracts GPU attributes (`gpu.N.*`), batches to ClickHouse, upserts GPU registry to PostgreSQL; records usage metrics when multi-tenant
- `internal/store/` — ClickHouse (time-series spans + gpu_metrics) and PostgreSQL (GPU registry) stores; all queries filter by `tenant_id`
- `internal/api/` — REST endpoints for traces, GPUs, analytics, admin (tenant/key management)
- `internal/config/` — YAML config with env var overrides (`AXONIZE_CONFIG` or default `config.yaml`)
- `internal/tenant/` — Multi-tenant support: context propagation (`tenant.go`), API key → tenant_id resolution with 5-min cache (`resolver.go`), usage metering for hybrid billing (`usage.go`)

### Server interface pattern
The Go server uses interface-based dependency injection. API handlers depend on query interfaces (`TraceQuerier`, `GPUQuerier`, `GPUMetricQuerier`, `AnalyticsQuerier`), not concrete stores. Ingest depends on `SpanWriter`, `GPURegistrar`, and `UsageRecorder`. When adding new query methods: define in the appropriate interface in `api/`, implement in `store/`, and the concrete store satisfies the interface implicitly.

### Multi-tenant context flow
Every request carries a `tenant_id` through Go's `context.Context`:
1. **gRPC**: Interceptor in `server.go` extracts Bearer token → resolves tenant → `tenant.WithTenantID(ctx, id)`
2. **HTTP**: Middleware in `router.go` does the same for REST API calls
3. **Handlers**: Extract via `tenant.FromContext(r.Context())` and pass to store queries
4. **Store**: All SQL queries include `WHERE tenant_id = ?`
5. **Ingest**: `convertRequest()` stamps `record.TenantID` from context; `registerGPUs()` propagates to GPU records

When `auth_mode = "static"` (default), tenant_id is always `"default"` — zero behavior change from single-tenant.

### Authentication modes
- **static** (default): Single API key via `AXONIZE_API_KEY`, all data under `tenant_id = "default"`
- **multi_tenant**: API key → tenant_id resolution via `api_keys` table (SHA-256 hash lookup, 5-min cache). Admin API protected by `AXONIZE_ADMIN_KEY`

### Databases
- **ClickHouse**: `spans`, `traces`, `gpu_metrics` tables. All include `tenant_id` column. Partitioned by day, TTL 7-90 days.
- **PostgreSQL**: 3-layer GPU identity model (all with composite `(tenant_id, ...)` PKs) + multi-tenant tables (`tenants`, `api_keys`, `usage_records`):
  1. `physical_gpus` — Immutable hardware (UUID, model, vendor, node)
  2. `compute_resources` — Logical unit (full GPU or MIG instance)
  3. `resource_contexts` — Runtime labels ("cuda:0", pod/container info)

This disambiguates MIG environments where every pod sees "cuda:0".

### Migrations
Raw SQL files in `migrations/{clickhouse,postgres}/`, applied alphabetically by `migrate.sh`. To add a new migration: create `NNN_description.sql` with the next sequence number. Use `IF NOT EXISTS` / `IF EXISTS` for idempotency.

### Docker services
- `clickhouse` — ports 8123 (HTTP), 9000 (native)
- `postgres` — port 5432
- `axonize-server` — gRPC :4317, HTTP :8080
- `dashboard` — port 3000

Environment variables for auth in docker-compose: `AXONIZE_API_KEY`, `AXONIZE_AUTH_MODE`, `AXONIZE_ADMIN_KEY`

## Key Conventions

- `ARCHITECTURE.md` is the single source of truth for schemas and data model decisions
- DB schema in `migrations/` must match ARCHITECTURE.md §6.2 (ClickHouse) and §6.3 (PostgreSQL)
- Conventional commits: `feat:`, `fix:`, `docs:`, `test:`, `refactor:`
- Python: `from __future__ import annotations` in all files; ruff line-length 100; mypy strict
- Python SDK pins `mypy>=1.10,<1.19` to avoid `librt` build failures on macOS
- Go: `log/slog` for structured logging; context-first function signatures; `fmt.Errorf("...: %w", err)` for error wrapping
- Go is not installed in the local dev environment — Go validation happens in CI
- Inference must never be affected by tracing failures — all exporter/GPU errors are swallowed with debug logging
- When wrapping a nil pointer in a Go interface, use an explicit nil check before assignment to avoid non-nil interface wrapping nil pointer (see `server.go` UsageRecorder pattern)
