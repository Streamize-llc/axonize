#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Defaults (overridable via environment)
POSTGRES_HOST="${POSTGRES_HOST:-localhost}"
POSTGRES_PORT="${POSTGRES_PORT:-5432}"
POSTGRES_DATABASE="${POSTGRES_DATABASE:-axonize}"
POSTGRES_USER="${POSTGRES_USER:-axonize}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-axonize}"

echo "=== Axonize Database Migration ==="

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
