# Server Deployment

## Docker Compose (Recommended)

The easiest way to run Axonize is with Docker Compose:

```bash
docker compose up -d
make migrate
```

This starts:
- **ClickHouse** (port 9000/8123) — time-series storage for spans and GPU metrics
- **PostgreSQL** (port 5432) — GPU registry
- **Axonize Server** (port 4317 gRPC, 8080 HTTP) — ingest and query
- **Dashboard** (port 3000) — web UI

## Configuration

The server loads configuration from environment variables. See `.env.example` for all options.

### Required Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `CLICKHOUSE_HOST` | `localhost` | ClickHouse hostname |
| `CLICKHOUSE_PORT` | `9000` | ClickHouse native port |
| `CLICKHOUSE_DATABASE` | `axonize` | Database name |
| `POSTGRES_HOST` | `localhost` | PostgreSQL hostname |
| `POSTGRES_PORT` | `5432` | PostgreSQL port |
| `POSTGRES_DATABASE` | `axonize` | Database name |
| `POSTGRES_USER` | `axonize` | PostgreSQL user |
| `POSTGRES_PASSWORD` | `axonize` | PostgreSQL password |

### Authentication Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `AXONIZE_API_KEY` | *(empty)* | Static API key for single-tenant mode |
| `AXONIZE_AUTH_MODE` | `static` | Auth mode: `static` (single key) or `multi_tenant` (per-tenant keys) |
| `AXONIZE_ADMIN_KEY` | *(empty)* | Admin API key (required for multi_tenant mode) |

### Server Ports

| Port | Protocol | Purpose |
|------|----------|---------|
| 4317 | gRPC | OTLP trace ingest |
| 8080 | HTTP | REST API + health checks |

### Auth Modes

**Static (default):** Set `AXONIZE_API_KEY` for simple single-key auth. All data is stored under `tenant_id = "default"`.

**Multi-tenant:** Set `AXONIZE_AUTH_MODE=multi_tenant` and `AXONIZE_ADMIN_KEY` to enable per-tenant isolation. Use the Admin API to create tenants and issue API keys. Each SDK instance authenticates with its own key, and data is isolated by tenant.

## Database Migrations

Migrations are raw SQL files applied in order:

```bash
make migrate
```

This runs `migrations/migrate.sh` which applies:
- `migrations/clickhouse/` — ClickHouse tables (spans, traces, gpu_metrics) with `tenant_id` columns
- `migrations/postgres/` — PostgreSQL tables (physical_gpus, compute_resources, resource_contexts, tenants, api_keys, usage_records)

## REST API Endpoints

### Health

```
GET /healthz     -> {"status": "ok"}
GET /readyz      -> {"status": "ready"}
```

### Traces

```
GET /api/v1/traces
    ?service_name=string    Filter by service
    &start=RFC3339          Start time
    &end=RFC3339            End time
    &limit=50               Results per page (max 1000)
    &offset=0               Pagination offset

GET /api/v1/traces/{trace_id}
    Returns full trace with nested span tree
```

### GPUs

```
GET /api/v1/gpus
    Returns all registered GPUs with latest metrics

GET /api/v1/gpus/{uuid}
    Returns GPU detail (physical + compute resource info)

GET /api/v1/gpus/{uuid}/metrics
    ?start=RFC3339          Start time (default: -1 hour)
    &end=RFC3339            End time (default: now)
    Returns GPU metric time series
```

### Analytics

```
GET /api/v1/analytics/overview
    ?start=RFC3339          Start time (default: -24 hours)
    &end=RFC3339            End time (default: now)
    Returns: total_traces, avg_latency_ms, error_rate,
             active_gpu_count, throughput_series, latency_series
```

### Admin API (multi_tenant mode)

Requires `Authorization: Bearer <ADMIN_KEY>` header.

```
POST /api/v1/admin/tenants
    Body: {"name": "Acme Corp", "plan": "pro"}
    Returns: {"tenant_id": "tn_...", "name": "...", "plan": "...", "created_at": "..."}

GET /api/v1/admin/tenants
    Returns: {"tenants": [...]}

POST /api/v1/admin/tenants/{id}/keys
    Body: {"name": "production", "scopes": "ingest,read"}
    Returns: {"key": "ax_live_...", "key_prefix": "...", ...}
    Note: Raw key is shown only at creation time

DELETE /api/v1/admin/tenants/{id}/keys/{prefix}
    Revokes the API key

GET /api/v1/admin/tenants/{id}/usage
    Returns today's usage: {"span_count": N, "gpu_seconds": N}
```

## Production Considerations

### Resource Limits

Set resource limits in Docker Compose for production:

```yaml
services:
  clickhouse:
    deploy:
      resources:
        limits:
          memory: 4G
  postgres:
    deploy:
      resources:
        limits:
          memory: 512M
  axonize-server:
    deploy:
      resources:
        limits:
          memory: 512M
```

### Data Retention

ClickHouse tables use TTL for automatic data cleanup:
- `spans`: 90-day retention
- `gpu_metrics`: 30-day retention

### Backup

- **ClickHouse**: Use `clickhouse-backup` or snapshot the data volume
- **PostgreSQL**: Standard `pg_dump` for the GPU registry
