#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Defaults (overridable via environment)
CLICKHOUSE_HOST="${CLICKHOUSE_HOST:-localhost}"
CLICKHOUSE_PORT="${CLICKHOUSE_PORT:-9000}"
CLICKHOUSE_DATABASE="${CLICKHOUSE_DATABASE:-axonize}"

POSTGRES_HOST="${POSTGRES_HOST:-localhost}"
POSTGRES_PORT="${POSTGRES_PORT:-5432}"
POSTGRES_DATABASE="${POSTGRES_DATABASE:-axonize}"
POSTGRES_USER="${POSTGRES_USER:-axonize}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-axonize}"

echo "=== Axonize Database Migration ==="

# ClickHouse migrations
echo ""
echo "--- ClickHouse migrations ---"
for f in "$SCRIPT_DIR"/clickhouse/*.sql; do
    echo "Applying $(basename "$f")..."
    clickhouse-client \
        --host "$CLICKHOUSE_HOST" \
        --port "$CLICKHOUSE_PORT" \
        --database "$CLICKHOUSE_DATABASE" \
        --multiquery \
        < "$f"
    echo "  Done."
done

# PostgreSQL migrations
echo ""
echo "--- PostgreSQL migrations ---"
export PGPASSWORD="$POSTGRES_PASSWORD"
for f in "$SCRIPT_DIR"/postgres/*.sql; do
    echo "Applying $(basename "$f")..."
    psql \
        -h "$POSTGRES_HOST" \
        -p "$POSTGRES_PORT" \
        -U "$POSTGRES_USER" \
        -d "$POSTGRES_DATABASE" \
        -f "$f"
    echo "  Done."
done

echo ""
echo "=== All migrations applied ==="
