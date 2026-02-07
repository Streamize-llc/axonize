package tenant

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// UsageMeter records span and GPU usage for hybrid billing.
type UsageMeter struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewUsageMeter creates a new usage meter.
func NewUsageMeter(pool *pgxpool.Pool, logger *slog.Logger) *UsageMeter {
	return &UsageMeter{pool: pool, logger: logger}
}

// RecordSpans increments the span count for the current day.
func (m *UsageMeter) RecordSpans(ctx context.Context, tenantID string, count int) {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	tomorrow := today.Add(24 * time.Hour)

	_, err := m.pool.Exec(ctx, `
		INSERT INTO usage_records (tenant_id, period_start, period_end, span_count)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (tenant_id, period_start)
		DO UPDATE SET span_count = usage_records.span_count + $4
	`, tenantID, today, tomorrow, count)
	if err != nil {
		m.logger.Debug("failed to record span usage", "tenant_id", tenantID, "error", err)
	}
}

// RecordGPUSeconds increments GPU seconds for the current day.
func (m *UsageMeter) RecordGPUSeconds(ctx context.Context, tenantID string, seconds int64) {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	tomorrow := today.Add(24 * time.Hour)

	_, err := m.pool.Exec(ctx, `
		INSERT INTO usage_records (tenant_id, period_start, period_end, gpu_seconds)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (tenant_id, period_start)
		DO UPDATE SET gpu_seconds = usage_records.gpu_seconds + $4
	`, tenantID, today, tomorrow, seconds)
	if err != nil {
		m.logger.Debug("failed to record gpu usage", "tenant_id", tenantID, "error", err)
	}
}

// GetUsage returns today's usage for a tenant.
func (m *UsageMeter) GetUsage(ctx context.Context, tenantID string) (spans int64, gpuSec int64, err error) {
	today := time.Now().UTC().Truncate(24 * time.Hour)

	err = m.pool.QueryRow(ctx, `
		SELECT COALESCE(span_count, 0), COALESCE(gpu_seconds, 0)
		FROM usage_records
		WHERE tenant_id = $1 AND period_start = $2
	`, tenantID, today).Scan(&spans, &gpuSec)
	if err != nil {
		// No record yet means zero usage
		if err.Error() == "no rows in result set" {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	return spans, gpuSec, nil
}
