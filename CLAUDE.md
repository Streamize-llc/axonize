# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## General Behavior

**Bias toward ACTION over PLANNING.** When given an implementation plan, start executing immediately rather than creating additional planning documents. If analysis is needed, keep it brief and transition to code changes within the first 2-3 messages. I value working code over comprehensive plans.

**When the user asks to scope work to a specific directory or project, stay strictly within that scope.** Do not scan or modify sibling projects unless explicitly asked.

## Project Context

Axonize is an observability platform for AI inference workloads. It sits between infrastructure monitoring (Grafana/Prometheus) and LLM service tracing (Langfuse/LangSmith), focusing on inference-level GPU metrics and performance tracking.

**Primary languages**: Python (SDK), Go (server). Dashboard lives in a separate repo (`axonize-web`). Single PostgreSQL database for all data (spans, metrics, GPU registry). The goal is a unified platform with multi-vendor GPU support (NVIDIA + Apple Silicon).

## Commands

### Development
```bash
make dev              # Start PostgreSQL container only
make dev-all          # Start all services (DBs + server)
make migrate          # Apply DB migrations (requires psql)
make clean            # Stop containers, remove volumes and build artifacts
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
SDK (Python) → gRPC (OTLP) → Server (Go) → PostgreSQL (spans, metrics, GPU registry)
                                          → REST API → axonize-web (separate repo)
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
- `internal/server.go` — Orchestrates gRPC + HTTP listeners, wires auth interceptors based on `auth_mode`, starts retention cleanup
- `internal/auth/auth.go` — JWT generation/validation (HS256) + bcrypt password hashing
- `internal/ingest/handler.go` — OTLP gRPC handler: parses protobuf spans, extracts GPU attributes (`gpu.N.*`), batches to PostgreSQL, upserts GPU registry; records usage metrics when multi-tenant
- `internal/store/` — PostgreSQL store for all data (spans, gpu_metrics, GPU registry, tenants); all queries filter by `tenant_id`; retention cleanup goroutine
- `internal/api/` — REST endpoints for traces, GPUs, analytics, auth (signup/login), admin (tenant/key management)
- `internal/config/` — YAML config with env var overrides (`AXONIZE_CONFIG` or default `config.yaml`)
- `internal/tenant/` — Multi-tenant support: context propagation (`tenant.go`), API key → tenant_id resolution with 5-min cache (`resolver.go`), usage metering for hybrid billing (`usage.go`)

### Server interface pattern
The Go server uses interface-based dependency injection. API handlers depend on query interfaces (`TraceQuerier`, `GPUQuerier`, `GPUMetricQuerier`, `AnalyticsQuerier`), not concrete stores. Ingest depends on `SpanWriter`, `GPURegistrar`, and `UsageRecorder`. All interfaces are implemented by `PostgresStore`. When adding new query methods: define in the appropriate interface in `api/`, implement in `store/postgres.go`.

### Multi-tenant context flow
Every request carries a `tenant_id` through Go's `context.Context`:
1. **gRPC**: Interceptor in `server.go` extracts Bearer token → resolves tenant → `tenant.WithTenantID(ctx, id)`
2. **HTTP**: Middleware in `router.go` does the same for REST API calls
3. **Handlers**: Extract via `tenant.FromContext(r.Context())` and pass to store queries
4. **Store**: All SQL queries include `WHERE tenant_id = ?`
5. **Ingest**: `convertRequest()` stamps `record.TenantID` from context; `registerGPUs()` propagates to GPU records

When `auth_mode = "static"` (default), tenant_id is always `"default"` — zero behavior change from single-tenant.

### Authentication
The server supports three auth mechanisms, all via `Authorization: Bearer <token>`:

- **Static API key** (`auth_mode = "static"`, default): Single key via `AXONIZE_API_KEY`, all data under `tenant_id = "default"`. Best for self-hosted single-user.
- **Multi-tenant API key** (`auth_mode = "multi_tenant"`): API key → tenant_id resolution via `api_keys` table (SHA-256 hash lookup, 5-min cache). Admin API protected by `AXONIZE_ADMIN_KEY`.
- **JWT user auth**: When `AXONIZE_JWT_SECRET` is set, enables `/api/v1/auth/*` endpoints (signup, login, me, logout). Signup auto-creates tenant + API key. The `jwtOrAPIKeyMiddleware` distinguishes JWT (3 dot-separated parts) from API keys (`ax_live_` prefix) automatically — both SDK and dashboard use the same Bearer token pattern.

Auth endpoints (`internal/api/auth.go`): `POST /auth/signup`, `POST /auth/login`, `GET /auth/me`, `POST /auth/logout`. Password hashing via bcrypt, JWT via HS256 with 24h expiry (`internal/auth/auth.go`).

### Database (PostgreSQL only)
Single PostgreSQL database holds all data:
- **Spans/Metrics**: `spans` table (30-day retention), `gpu_metrics` table (7-day retention). Retention enforced by server-side cleanup goroutine.
- **GPU Registry**: 3-layer identity model (all with composite `(tenant_id, ...)` PKs):
  1. `physical_gpus` — Immutable hardware (UUID, model, vendor, node)
  2. `compute_resources` — Logical unit (full GPU or MIG instance)
  3. `resource_contexts` — Runtime labels ("cuda:0", pod/container info)
- **Auth & Multi-tenant**: `users` (email/password), `tenants`, `api_keys`, `usage_records`

This disambiguates MIG environments where every pod sees "cuda:0".

### Migrations
Raw SQL files in `migrations/postgres/`, applied alphabetically by `migrate.sh`. To add a new migration: create `NNN_description.sql` with the next sequence number. Use `IF NOT EXISTS` / `IF EXISTS` for idempotency.

### Docker services
- `postgres` — port 5432
- `axonize-server` — gRPC :4317, HTTP :8080

Environment variables for auth in docker-compose: `AXONIZE_API_KEY`, `AXONIZE_AUTH_MODE`, `AXONIZE_ADMIN_KEY`, `AXONIZE_JWT_SECRET`

### Deployment
- **Server**: Fly.io (`fly.toml` at repo root, builds `server/Dockerfile`, `axonize` app in `streamize` org, `iad` region)
- **Dashboard**: Vercel (separate repo `axonize-web`, `streamize` team)
- **Database**: Fly Postgres (`axonize-db` app, internal network `axonize-db.internal:5432`). For local access: `fly proxy 15432:5432 --app axonize-db`
- Self-hosted: Docker PostgreSQL via `docker compose up -d` (same schema, same server binary)
- PgBouncer compatibility: `pgx` configured with `QueryExecModeSimpleProtocol` (for external poolers)

## Code Deletion & Cleanup

**When asked to delete or remove code, ALWAYS confirm the specific list of items to delete/keep with the user BEFORE making changes.** Never assume something is unused without verification. Present a deletion list and explicitly ask if any items should be preserved.

## Communication Style

**When asking the user clarifying questions, ask them ONE AT A TIME, not in parallel batches.** Wait for each answer before asking the next question. This allows for detailed, focused responses rather than overwhelming with multiple questions at once.

## Git & Commits

**When committing changes, ONLY include files relevant to the current task.** Review staged files before committing and exclude unrelated changes. Use `git status` to verify what's being committed.

## Key Conventions

- `ARCHITECTURE.md` is the single source of truth for schemas and data model decisions
- DB schema in `migrations/postgres/` must match ARCHITECTURE.md
- Conventional commits: `feat:`, `fix:`, `docs:`, `test:`, `refactor:`
- Python: `from __future__ import annotations` in all files; ruff line-length 100; mypy strict
- Python SDK pins `mypy>=1.10,<1.19` to avoid `librt` build failures on macOS
- Go: `log/slog` for structured logging; context-first function signatures; `fmt.Errorf("...: %w", err)` for error wrapping
- Go is not installed in the local dev environment — Go validation happens in CI
- Inference must never be affected by tracing failures — all exporter/GPU errors are swallowed with debug logging
- When wrapping a nil pointer in a Go interface, use an explicit nil check before assignment to avoid non-nil interface wrapping nil pointer (see `server.go` UsageRecorder pattern)
