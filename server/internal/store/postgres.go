package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/axonize/server/internal/config"
)

// PostgresStore manages the PostgreSQL connection and GPU registry operations.
type PostgresStore struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewPostgresStore creates a new PostgreSQL connection pool.
func NewPostgresStore(cfg config.PostgreSQLConfig, logger *slog.Logger) (*PostgresStore, error) {
	sslmode := cfg.SSLMode
	if sslmode == "" {
		sslmode = "disable"
	}
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database, sslmode,
	)

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres parse config: %w", err)
	}
	// Use simple protocol for PgBouncer (Supabase Transaction Pooler) compatibility.
	poolCfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return nil, fmt.Errorf("postgres connect: %w", err)
	}

	return &PostgresStore{pool: pool, logger: logger}, nil
}

// Pool exposes the underlying connection pool for use by other components (e.g., tenant.Resolver).
func (s *PostgresStore) Pool() *pgxpool.Pool {
	return s.pool
}

// Ping checks the PostgreSQL connection.
func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// Close closes the connection pool.
func (s *PostgresStore) Close() {
	s.pool.Close()
}

// UpsertPhysicalGPU inserts or updates a physical GPU record.
func (s *PostgresStore) UpsertPhysicalGPU(ctx context.Context, gpu PhysicalGPURecord) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO physical_gpus (tenant_id, uuid, model, vendor, memory_total_gb, node_id, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (tenant_id, uuid)
		DO UPDATE SET last_seen_at = NOW(), model = EXCLUDED.model, node_id = EXCLUDED.node_id
	`, gpu.TenantID, gpu.UUID, gpu.Model, gpu.Vendor, gpu.MemoryTotalGB, gpu.NodeID)
	if err != nil {
		return fmt.Errorf("upsert physical gpu: %w", err)
	}
	return nil
}

// UpsertComputeResource inserts or updates a compute resource record.
func (s *PostgresStore) UpsertComputeResource(ctx context.Context, res ComputeResourceRecord) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO compute_resources (tenant_id, resource_uuid, physical_gpu_uuid, resource_type, memory_gb, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (tenant_id, resource_uuid)
		DO UPDATE SET last_seen_at = NOW(), resource_type = EXCLUDED.resource_type
	`, res.TenantID, res.ResourceUUID, res.PhysicalUUID, res.ResourceType, res.MemoryGB)
	if err != nil {
		return fmt.Errorf("upsert compute resource: %w", err)
	}
	return nil
}

// UpsertResourceContext inserts or updates a resource context (runtime label mapping).
func (s *PostgresStore) UpsertResourceContext(ctx context.Context, rc ResourceContextRecord) error {
	// context_id = resource_uuid + user_label (deterministic)
	contextID := rc.ResourceUUID + ":" + rc.UserLabel
	_, err := s.pool.Exec(ctx, `
		INSERT INTO resource_contexts (tenant_id, context_id, resource_uuid, user_label, hostname, attached_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (tenant_id, context_id)
		DO UPDATE SET hostname = EXCLUDED.hostname, detached_at = NULL
	`, rc.TenantID, contextID, rc.ResourceUUID, rc.UserLabel, rc.Hostname)
	if err != nil {
		return fmt.Errorf("upsert resource context: %w", err)
	}
	return nil
}

// ListGPUs returns a summary of all registered GPUs for a tenant.
func (s *PostgresStore) ListGPUs(ctx context.Context, tenantID string) ([]GPUSummary, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT cr.resource_uuid, cr.physical_gpu_uuid, pg.model, cr.resource_type, pg.node_id
		FROM compute_resources cr
		JOIN physical_gpus pg ON pg.tenant_id = cr.tenant_id AND pg.uuid = cr.physical_gpu_uuid
		WHERE cr.tenant_id = $1
		ORDER BY pg.node_id, cr.resource_uuid
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list gpus: %w", err)
	}
	defer rows.Close()

	var gpus []GPUSummary
	for rows.Next() {
		var g GPUSummary
		if err := rows.Scan(&g.ResourceUUID, &g.PhysicalUUID, &g.Model, &g.ResourceType, &g.NodeID); err != nil {
			return nil, fmt.Errorf("scan gpu: %w", err)
		}
		gpus = append(gpus, g)
	}
	return gpus, nil
}

// GetGPU returns details for a single GPU by resource UUID within a tenant.
func (s *PostgresStore) GetGPU(ctx context.Context, tenantID, uuid string) (*GPUDetail, error) {
	var g GPUDetail
	err := s.pool.QueryRow(ctx, `
		SELECT cr.resource_uuid, cr.physical_gpu_uuid, pg.model, cr.resource_type, pg.node_id,
		       cr.created_at, cr.last_seen_at
		FROM compute_resources cr
		JOIN physical_gpus pg ON pg.tenant_id = cr.tenant_id AND pg.uuid = cr.physical_gpu_uuid
		WHERE cr.tenant_id = $1 AND cr.resource_uuid = $2
	`, tenantID, uuid).Scan(&g.ResourceUUID, &g.PhysicalUUID, &g.Model, &g.ResourceType, &g.NodeID,
		&g.FirstSeen, &g.LastSeen)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get gpu: %w", err)
	}
	return &g, nil
}

// ---------------------------------------------------------------------------
// Span & GPU metric writes (implements ingest.SpanWriter)
// ---------------------------------------------------------------------------

// InsertSpans inserts a batch of span records into PostgreSQL.
func (s *PostgresStore) InsertSpans(ctx context.Context, spans []SpanRecord) error {
	if len(spans) == 0 {
		return nil
	}

	query := `INSERT INTO spans (
		tenant_id, trace_id, span_id, parent_span_id,
		name, service_name, environment,
		start_time, end_time, duration_ms,
		model_name, model_version, inference_type,
		tokens_input, tokens_output, tokens_per_second, ttft_ms,
		diffusion_steps, cfg_scale,
		gpu_resource_uuids, gpu_physical_uuids, gpu_models, gpu_vendors, gpu_node_ids,
		gpu_memory_used_gb, gpu_utilization, gpu_power_watts,
		cost_usd, status, error_message, attributes
	) VALUES (
		$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,
		$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31
	) ON CONFLICT (tenant_id, trace_id, span_id) DO NOTHING`

	batch := &pgx.Batch{}
	for _, span := range spans {
		attrsJSON, _ := json.Marshal(span.Attributes)

		// Ensure non-nil slices for PostgreSQL array columns
		gpuResUUIDs := span.GpuResourceUUIDs
		if gpuResUUIDs == nil {
			gpuResUUIDs = []string{}
		}
		gpuPhysUUIDs := span.GpuPhysicalUUIDs
		if gpuPhysUUIDs == nil {
			gpuPhysUUIDs = []string{}
		}
		gpuModels := span.GpuModels
		if gpuModels == nil {
			gpuModels = []string{}
		}
		gpuVendors := span.GpuVendors
		if gpuVendors == nil {
			gpuVendors = []string{}
		}
		gpuNodeIDs := span.GpuNodeIDs
		if gpuNodeIDs == nil {
			gpuNodeIDs = []string{}
		}
		gpuMemUsed := span.GpuMemoryUsedGB
		if gpuMemUsed == nil {
			gpuMemUsed = []float32{}
		}
		gpuUtil := span.GpuUtilization
		if gpuUtil == nil {
			gpuUtil = []float32{}
		}
		gpuPower := make([]int32, len(span.GpuPowerWatts))
		for i, v := range span.GpuPowerWatts {
			gpuPower[i] = int32(v)
		}

		batch.Queue(query,
			span.TenantID, span.TraceID, span.SpanID, span.ParentSpanID,
			span.Name, span.ServiceName, span.Environment,
			span.StartTime, span.EndTime, span.DurationMs,
			span.ModelName, span.ModelVersion, span.InferenceType,
			span.TokensInput, span.TokensOutput, span.TokensPerSecond, span.TtftMs,
			span.DiffusionSteps, span.CfgScale,
			gpuResUUIDs, gpuPhysUUIDs, gpuModels, gpuVendors, gpuNodeIDs,
			gpuMemUsed, gpuUtil, gpuPower,
			span.CostUSD, span.Status, span.ErrorMessage, attrsJSON,
		)
	}

	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()

	for range spans {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("insert span: %w", err)
		}
	}

	s.logger.Debug("inserted spans", "count", len(spans))
	return nil
}

// InsertGPUMetrics extracts GPU data from span records and inserts into gpu_metrics table.
func (s *PostgresStore) InsertGPUMetrics(ctx context.Context, spans []SpanRecord) error {
	total := 0
	for _, span := range spans {
		total += len(span.GpuResourceUUIDs)
	}
	if total == 0 {
		return nil
	}

	query := `INSERT INTO gpu_metrics (
		tenant_id, timestamp, resource_uuid, physical_gpu_uuid, node_id,
		utilization, memory_used_gb, memory_total_gb,
		temperature_celsius, power_watts, clock_mhz,
		active_spans
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`

	batch := &pgx.Batch{}
	for _, span := range spans {
		for i, resourceUUID := range span.GpuResourceUUIDs {
			var physicalUUID, nodeID string
			if i < len(span.GpuPhysicalUUIDs) {
				physicalUUID = span.GpuPhysicalUUIDs[i]
			}
			if i < len(span.GpuNodeIDs) {
				nodeID = span.GpuNodeIDs[i]
			}
			var util, memUsed, memTotal float32
			if i < len(span.GpuUtilization) {
				util = span.GpuUtilization[i]
			}
			if i < len(span.GpuMemoryUsedGB) {
				memUsed = span.GpuMemoryUsedGB[i]
			}
			if i < len(span.GpuMemoryTotalGB) {
				memTotal = span.GpuMemoryTotalGB[i]
			}
			var temp int16
			if i < len(span.GpuTemperatureCels) {
				temp = int16(span.GpuTemperatureCels[i])
			}
			var power, clock int16
			if i < len(span.GpuPowerWatts) {
				power = int16(span.GpuPowerWatts[i])
			}
			if i < len(span.GpuClockMHz) {
				clock = int16(span.GpuClockMHz[i])
			}

			batch.Queue(query,
				span.TenantID,
				span.StartTime, resourceUUID, physicalUUID, nodeID,
				util, memUsed, memTotal,
				temp, power, clock,
				int16(1),
			)
		}
	}

	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()

	for i := 0; i < total; i++ {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("insert gpu metric: %w", err)
		}
	}

	s.logger.Debug("inserted gpu metrics", "count", total)
	return nil
}

// ---------------------------------------------------------------------------
// Query methods (implements api.TraceQuerier, api.GPUMetricQuerier, api.AnalyticsQuerier)
// ---------------------------------------------------------------------------

// QueryGPUMetrics returns GPU metric time series for a given resource UUID.
func (s *PostgresStore) QueryGPUMetrics(ctx context.Context, tenantID, uuid string, start, end time.Time) ([]GPUMetricRow, error) {
	query := `
		SELECT timestamp, resource_uuid, utilization, memory_used_gb, power_watts
		FROM gpu_metrics
		WHERE tenant_id = $1
		  AND resource_uuid = $2
		  AND timestamp >= $3
		  AND timestamp <= $4
		ORDER BY timestamp ASC
	`

	rows, err := s.pool.Query(ctx, query, tenantID, uuid, start, end)
	if err != nil {
		return nil, fmt.Errorf("query gpu metrics: %w", err)
	}
	defer rows.Close()

	var metrics []GPUMetricRow
	for rows.Next() {
		var m GPUMetricRow
		if err := rows.Scan(&m.Timestamp, &m.ResourceUUID, &m.Utilization, &m.MemoryUsedGB, &m.PowerWatts); err != nil {
			return nil, fmt.Errorf("scan gpu metric: %w", err)
		}
		metrics = append(metrics, m)
	}
	return metrics, nil
}

// QueryAnalyticsOverview returns aggregated analytics for the dashboard overview.
func (s *PostgresStore) QueryAnalyticsOverview(ctx context.Context, tenantID string, start, end time.Time) (*AnalyticsOverview, error) {
	overview := &AnalyticsOverview{}

	// 1. Summary stats
	err := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(DISTINCT trace_id) AS total_traces,
			COALESCE(AVG(duration_ms), 0) AS avg_latency_ms,
			CASE WHEN COUNT(*) > 0
				THEN COUNT(*) FILTER (WHERE status = 'error')::float / COUNT(*)
				ELSE 0
			END AS error_rate,
			(SELECT COUNT(DISTINCT u) FROM spans s2, unnest(s2.gpu_resource_uuids) AS u
			 WHERE s2.tenant_id = $1 AND s2.start_time >= $2 AND s2.start_time <= $3) AS active_gpu_count
		FROM spans
		WHERE tenant_id = $1 AND start_time >= $2 AND start_time <= $3
	`, tenantID, start, end).Scan(
		&overview.TotalTraces,
		&overview.AvgLatencyMs,
		&overview.ErrorRate,
		&overview.ActiveGPUCount,
	)
	if err != nil {
		return nil, fmt.Errorf("analytics summary: %w", err)
	}

	// 2. Throughput series (per hour buckets)
	rows, err := s.pool.Query(ctx, `
		SELECT
			date_trunc('hour', start_time) AS bucket,
			COUNT(DISTINCT trace_id) AS cnt
		FROM spans
		WHERE tenant_id = $1 AND start_time >= $2 AND start_time <= $3
		GROUP BY bucket
		ORDER BY bucket ASC
	`, tenantID, start, end)
	if err != nil {
		return nil, fmt.Errorf("analytics throughput: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var pt ThroughputPoint
		if err := rows.Scan(&pt.Timestamp, &pt.Count); err != nil {
			return nil, fmt.Errorf("scan throughput: %w", err)
		}
		overview.ThroughputSeries = append(overview.ThroughputSeries, pt)
	}

	// 3. Latency percentiles series (per hour buckets)
	rows2, err := s.pool.Query(ctx, `
		SELECT
			date_trunc('hour', start_time) AS bucket,
			PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY duration_ms) AS p50,
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration_ms) AS p95,
			PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY duration_ms) AS p99
		FROM spans
		WHERE tenant_id = $1 AND start_time >= $2 AND start_time <= $3
		GROUP BY bucket
		ORDER BY bucket ASC
	`, tenantID, start, end)
	if err != nil {
		return nil, fmt.Errorf("analytics latency: %w", err)
	}
	defer rows2.Close()

	for rows2.Next() {
		var pt LatencyPoint
		if err := rows2.Scan(&pt.Timestamp, &pt.P50Ms, &pt.P95Ms, &pt.P99Ms); err != nil {
			return nil, fmt.Errorf("scan latency: %w", err)
		}
		overview.LatencySeries = append(overview.LatencySeries, pt)
	}

	// Ensure non-nil slices for JSON
	if overview.ThroughputSeries == nil {
		overview.ThroughputSeries = []ThroughputPoint{}
	}
	if overview.LatencySeries == nil {
		overview.LatencySeries = []LatencyPoint{}
	}

	return overview, nil
}

// QueryTraces returns a paginated list of trace summaries.
func (s *PostgresStore) QueryTraces(ctx context.Context, f TraceFilter) ([]TraceSummary, int, error) {
	// Build WHERE clause
	where := "WHERE tenant_id = $1"
	args := []interface{}{f.TenantID}
	paramIdx := 2

	if f.ServiceName != nil {
		where += fmt.Sprintf(" AND service_name = $%d", paramIdx)
		args = append(args, *f.ServiceName)
		paramIdx++
	}
	if f.StartTime != nil {
		where += fmt.Sprintf(" AND start_time >= $%d", paramIdx)
		args = append(args, *f.StartTime)
		paramIdx++
	}
	if f.EndTime != nil {
		where += fmt.Sprintf(" AND start_time <= $%d", paramIdx)
		args = append(args, *f.EndTime)
		paramIdx++
	}

	// Count query
	countQuery := fmt.Sprintf(`SELECT COUNT(DISTINCT trace_id) FROM spans %s`, where)
	var total int
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count traces: %w", err)
	}

	// Data query
	query := fmt.Sprintf(`
		SELECT
			trace_id,
			min(start_time) AS start_time,
			max(end_time) AS end_time,
			EXTRACT(EPOCH FROM (max(end_time) - min(start_time))) * 1000 AS duration_ms,
			(array_agg(service_name))[1] AS service_name,
			(array_agg(environment))[1] AS environment,
			COUNT(*) AS span_count,
			COUNT(*) FILTER (WHERE status = 'error') AS error_count
		FROM spans
		%s
		GROUP BY trace_id
		ORDER BY min(start_time) DESC
		LIMIT $%d OFFSET $%d
	`, where, paramIdx, paramIdx+1)

	args = append(args, f.Limit, f.Offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query traces: %w", err)
	}
	defer rows.Close()

	var traces []TraceSummary
	for rows.Next() {
		var t TraceSummary
		if err := rows.Scan(
			&t.TraceID, &t.StartTime, &t.EndTime, &t.DurationMs,
			&t.ServiceName, &t.Environment, &t.SpanCount, &t.ErrorCount,
		); err != nil {
			return nil, 0, fmt.Errorf("scan trace: %w", err)
		}
		traces = append(traces, t)
	}

	return traces, total, nil
}

// QueryTraceByID returns the full detail of a single trace, including a span tree.
func (s *PostgresStore) QueryTraceByID(ctx context.Context, tenantID string, traceID string) (*TraceDetail, error) {
	query := `
		SELECT span_id, parent_span_id, name, service_name, environment,
		       start_time, end_time, duration_ms,
		       status, error_message, attributes
		FROM spans
		WHERE tenant_id = $1 AND trace_id = $2
		ORDER BY start_time ASC
	`

	rows, err := s.pool.Query(ctx, query, tenantID, traceID)
	if err != nil {
		return nil, fmt.Errorf("query trace spans: %w", err)
	}
	defer rows.Close()

	spanMap := make(map[string]*SpanDetail)
	var allSpans []*SpanDetail
	var traceServiceName, traceEnvironment string

	for rows.Next() {
		var sd SpanDetail
		var svcName, env string
		var attrsJSON []byte
		if err := rows.Scan(
			&sd.SpanID, &sd.ParentSpanID, &sd.Name,
			&svcName, &env,
			&sd.StartTime, &sd.EndTime, &sd.DurationMs,
			&sd.Status, &sd.ErrorMessage, &attrsJSON,
		); err != nil {
			return nil, fmt.Errorf("scan span: %w", err)
		}
		if attrsJSON != nil {
			_ = json.Unmarshal(attrsJSON, &sd.Attributes)
		}
		if len(allSpans) == 0 {
			traceServiceName = svcName
			traceEnvironment = env
		}
		sd.Children = make([]*SpanDetail, 0)
		spanMap[sd.SpanID] = &sd
		allSpans = append(allSpans, &sd)
	}

	if len(allSpans) == 0 {
		return nil, nil
	}

	// Build tree
	var roots []*SpanDetail
	for _, sd := range allSpans {
		if sd.ParentSpanID != nil {
			if parent, ok := spanMap[*sd.ParentSpanID]; ok {
				parent.Children = append(parent.Children, sd)
				continue
			}
		}
		roots = append(roots, sd)
	}

	// Aggregate trace-level info
	first := allSpans[0]
	last := allSpans[0]
	var errorCount uint64
	for _, sd := range allSpans {
		if sd.StartTime.Before(first.StartTime) {
			first = sd
		}
		if sd.EndTime.After(last.EndTime) {
			last = sd
		}
		if sd.Status == "error" {
			errorCount++
		}
	}

	detail := &TraceDetail{
		TraceID:     traceID,
		StartTime:   first.StartTime,
		EndTime:     last.EndTime,
		DurationMs:  float64(last.EndTime.Sub(first.StartTime).Milliseconds()),
		ServiceName: traceServiceName,
		Environment: traceEnvironment,
		SpanCount:   uint64(len(allSpans)),
		ErrorCount:  errorCount,
		Spans:       roots,
	}

	return detail, nil
}

// ---------------------------------------------------------------------------
// Retention cleanup
// ---------------------------------------------------------------------------

// StartRetentionCleanup starts a background goroutine that periodically
// deletes old spans and gpu_metrics rows.
func (s *PostgresStore) StartRetentionCleanup(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.runRetentionCleanup()
			}
		}
	}()
}

func (s *PostgresStore) runRetentionCleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := s.pool.Exec(ctx, `DELETE FROM spans WHERE start_time < NOW() - INTERVAL '30 days'`); err != nil {
		s.logger.Error("retention cleanup: spans", "error", err)
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM gpu_metrics WHERE timestamp < NOW() - INTERVAL '7 days'`); err != nil {
		s.logger.Error("retention cleanup: gpu_metrics", "error", err)
	}
	s.logger.Debug("retention cleanup completed")
}

// Ensure PostgresStore satisfies compile-time interfaces.
var _ interface {
	UpsertPhysicalGPU(ctx context.Context, gpu PhysicalGPURecord) error
	UpsertComputeResource(ctx context.Context, res ComputeResourceRecord) error
	UpsertResourceContext(ctx context.Context, rc ResourceContextRecord) error
	InsertSpans(ctx context.Context, spans []SpanRecord) error
	InsertGPUMetrics(ctx context.Context, spans []SpanRecord) error
} = (*PostgresStore)(nil)
