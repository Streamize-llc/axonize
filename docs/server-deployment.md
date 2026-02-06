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

### Server Ports

| Port | Protocol | Purpose |
|------|----------|---------|
| 4317 | gRPC | OTLP trace ingest |
| 8080 | HTTP | REST API + health checks |

## Database Migrations

Migrations are raw SQL files applied in order:

```bash
make migrate
```

This runs `migrations/migrate.sh` which applies:
- `migrations/clickhouse/` — ClickHouse tables (spans, gpu_metrics)
- `migrations/postgres/` — PostgreSQL tables (physical_gpus, compute_resources, resource_contexts)

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
