package store

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/axonize/server/internal/config"
)

// ClickHouseStore manages the ClickHouse connection and query operations.
type ClickHouseStore struct {
	conn   driver.Conn
	logger *slog.Logger
}

// NewClickHouseStore creates a new ClickHouse connection.
func NewClickHouseStore(cfg config.ClickHouseConfig, logger *slog.Logger) (*ClickHouseStore, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)},
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.User,
			Password: cfg.Password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("clickhouse open: %w", err)
	}

	return &ClickHouseStore{conn: conn, logger: logger}, nil
}

// Ping checks the ClickHouse connection.
func (s *ClickHouseStore) Ping(ctx context.Context) error {
	return s.conn.Ping(ctx)
}

// Close closes the ClickHouse connection.
func (s *ClickHouseStore) Close() error {
	return s.conn.Close()
}

// InsertSpans inserts a batch of span records into ClickHouse.
func (s *ClickHouseStore) InsertSpans(ctx context.Context, spans []SpanRecord) error {
	if len(spans) == 0 {
		return nil
	}

	batch, err := s.conn.PrepareBatch(ctx, `INSERT INTO spans (
		trace_id, span_id, parent_span_id,
		name, service_name, environment,
		start_time, end_time, duration_ms,
		model_name, model_version, inference_type,
		tokens_input, tokens_output, tokens_per_second, ttft_ms,
		diffusion_steps, cfg_scale,
		gpu_resource_uuids, gpu_physical_uuids, gpu_models, gpu_node_ids,
		gpu_memory_used_gb, gpu_utilization, gpu_power_watts,
		cost_usd, status, error_message, attributes
	)`)
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}

	for _, span := range spans {
		// Ensure GPU slices are non-nil for ClickHouse Array columns
		gpuResUUIDs := span.GpuResourceUUIDs
		if gpuResUUIDs == nil { gpuResUUIDs = []string{} }
		gpuPhysUUIDs := span.GpuPhysicalUUIDs
		if gpuPhysUUIDs == nil { gpuPhysUUIDs = []string{} }
		gpuModels := span.GpuModels
		if gpuModels == nil { gpuModels = []string{} }
		gpuNodeIDs := span.GpuNodeIDs
		if gpuNodeIDs == nil { gpuNodeIDs = []string{} }
		gpuMemUsed := span.GpuMemoryUsedGB
		if gpuMemUsed == nil { gpuMemUsed = []float32{} }
		gpuUtil := span.GpuUtilization
		if gpuUtil == nil { gpuUtil = []float32{} }
		gpuPower := span.GpuPowerWatts
		if gpuPower == nil { gpuPower = []uint16{} }

		if err := batch.Append(
			span.TraceID, span.SpanID, span.ParentSpanID,
			span.Name, span.ServiceName, span.Environment,
			span.StartTime, span.EndTime, span.DurationMs,
			span.ModelName, span.ModelVersion, span.InferenceType,
			span.TokensInput, span.TokensOutput, span.TokensPerSecond, span.TtftMs,
			span.DiffusionSteps, span.CfgScale,
			gpuResUUIDs, gpuPhysUUIDs, gpuModels, gpuNodeIDs,
			gpuMemUsed, gpuUtil, gpuPower,
			span.CostUSD, span.Status, span.ErrorMessage, span.Attributes,
		); err != nil {
			return fmt.Errorf("append span: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("send batch: %w", err)
	}

	s.logger.Debug("inserted spans", "count", len(spans))
	return nil
}

// InsertGPUMetrics extracts GPU data from span records and inserts into gpu_metrics table.
func (s *ClickHouseStore) InsertGPUMetrics(ctx context.Context, spans []SpanRecord) error {
	// Count GPU data points
	total := 0
	for _, span := range spans {
		total += len(span.GpuResourceUUIDs)
	}
	if total == 0 {
		return nil
	}

	batch, err := s.conn.PrepareBatch(ctx, `INSERT INTO gpu_metrics (
		timestamp, resource_uuid, physical_gpu_uuid, node_id,
		utilization, memory_used_gb, memory_total_gb,
		temperature_celsius, power_watts, clock_mhz,
		active_spans
	)`)
	if err != nil {
		return fmt.Errorf("prepare gpu metrics batch: %w", err)
	}

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
			var temp uint8
			if i < len(span.GpuTemperatureCels) {
				temp = span.GpuTemperatureCels[i]
			}
			var power, clock uint16
			if i < len(span.GpuPowerWatts) {
				power = span.GpuPowerWatts[i]
			}
			if i < len(span.GpuClockMHz) {
				clock = span.GpuClockMHz[i]
			}

			if err := batch.Append(
				span.StartTime, resourceUUID, physicalUUID, nodeID,
				util, memUsed, memTotal,
				temp, power, clock,
				uint16(1), // active_spans: 1 per span
			); err != nil {
				return fmt.Errorf("append gpu metric: %w", err)
			}
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("send gpu metrics batch: %w", err)
	}

	s.logger.Debug("inserted gpu metrics", "count", total)
	return nil
}

// QueryGPUMetrics returns GPU metric time series for a given resource UUID.
func (s *ClickHouseStore) QueryGPUMetrics(ctx context.Context, uuid string, start, end time.Time) ([]GPUMetricRow, error) {
	query := `
		SELECT timestamp, resource_uuid, utilization, memory_used_gb, power_watts
		FROM gpu_metrics
		WHERE resource_uuid = ?
		  AND timestamp >= ?
		  AND timestamp <= ?
		ORDER BY timestamp ASC
	`

	rows, err := s.conn.Query(ctx, query, uuid, start, end)
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

// QueryTraces returns a paginated list of trace summaries.
func (s *ClickHouseStore) QueryTraces(ctx context.Context, f TraceFilter) ([]TraceSummary, int, error) {
	// Build WHERE clause
	where := "WHERE 1=1"
	args := make([]interface{}, 0)

	if f.ServiceName != nil {
		where += " AND service_name = ?"
		args = append(args, *f.ServiceName)
	}
	if f.StartTime != nil {
		where += " AND start_time >= ?"
		args = append(args, *f.StartTime)
	}
	if f.EndTime != nil {
		where += " AND start_time <= ?"
		args = append(args, *f.EndTime)
	}

	// Count query
	countQuery := fmt.Sprintf(`SELECT count(DISTINCT trace_id) FROM spans %s`, where)
	var total int
	if err := s.conn.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count traces: %w", err)
	}

	// Data query
	query := fmt.Sprintf(`
		SELECT
			trace_id,
			min(start_time) AS start_time,
			max(end_time) AS end_time,
			dateDiff('millisecond', min(start_time), max(end_time)) AS duration_ms,
			any(service_name) AS service_name,
			any(environment) AS environment,
			count() AS span_count,
			countIf(status = 'error') AS error_count
		FROM spans
		%s
		GROUP BY trace_id
		ORDER BY min(start_time) DESC
		LIMIT ? OFFSET ?
	`, where)

	args = append(args, f.Limit, f.Offset)

	rows, err := s.conn.Query(ctx, query, args...)
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
func (s *ClickHouseStore) QueryTraceByID(ctx context.Context, traceID string) (*TraceDetail, error) {
	query := `
		SELECT span_id, parent_span_id, name, start_time, end_time, duration_ms,
		       status, error_message, attributes
		FROM spans
		WHERE trace_id = ?
		ORDER BY start_time ASC
	`

	rows, err := s.conn.Query(ctx, query, traceID)
	if err != nil {
		return nil, fmt.Errorf("query trace spans: %w", err)
	}
	defer rows.Close()

	spanMap := make(map[string]*SpanDetail)
	var allSpans []*SpanDetail

	for rows.Next() {
		var sd SpanDetail
		if err := rows.Scan(
			&sd.SpanID, &sd.ParentSpanID, &sd.Name,
			&sd.StartTime, &sd.EndTime, &sd.DurationMs,
			&sd.Status, &sd.ErrorMessage, &sd.Attributes,
		); err != nil {
			return nil, fmt.Errorf("scan span: %w", err)
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
		ServiceName: first.Name,
		SpanCount:   uint64(len(allSpans)),
		ErrorCount:  errorCount,
		Spans:       roots,
	}

	return detail, nil
}
