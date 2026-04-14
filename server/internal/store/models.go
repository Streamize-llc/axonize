package store

import "time"

// SpanRecord maps 1:1 to the PostgreSQL spans table.
type SpanRecord struct {
	TenantID     string
	TraceID      string
	SpanID       string
	ParentSpanID *string

	Name        string
	ServiceName string
	Environment string

	StartTime  time.Time
	EndTime    time.Time
	DurationMs float64

	ModelName     *string
	ModelVersion  *string
	InferenceType *string

	TokensInput     *uint32
	TokensOutput    *uint32
	TokensPerSecond *float32
	TtftMs          *float32

	DiffusionSteps *uint16
	CfgScale       *float32

	GpuResourceUUIDs    []string
	GpuPhysicalUUIDs    []string
	GpuModels           []string
	GpuVendors          []string
	GpuNodeIDs          []string
	GpuResourceTypes    []string
	GpuUserLabels       []string
	GpuMemoryUsedGB     []float32
	GpuMemoryTotalGB    []float32
	GpuUtilization      []float32
	GpuTemperatureCels  []uint8
	GpuPowerWatts       []uint16
	GpuClockMHz         []uint16

	CostUSD *float64

	Status       string
	ErrorMessage *string

	Attributes map[string]string
}

// TraceSummary is returned by the trace list query.
type TraceSummary struct {
	TraceID     string    `json:"trace_id"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	DurationMs  float64   `json:"duration_ms"`
	ServiceName string    `json:"service_name"`
	Environment string    `json:"environment"`
	SpanCount   uint64    `json:"span_count"`
	ErrorCount  uint64    `json:"error_count"`
}

// SpanDetail is an individual span in a trace detail response.
type SpanDetail struct {
	SpanID       string            `json:"span_id"`
	ParentSpanID *string           `json:"parent_span_id,omitempty"`
	Name         string            `json:"name"`
	StartTime    time.Time         `json:"start_time"`
	EndTime      time.Time         `json:"end_time"`
	DurationMs   float64           `json:"duration_ms"`
	Status       string            `json:"status"`
	ErrorMessage *string           `json:"error_message,omitempty"`

	// AI-specific fields (first-class, not buried in attributes)
	ModelName       *string  `json:"model_name,omitempty"`
	InferenceType   *string  `json:"inference_type,omitempty"`
	TokensInput     *uint32  `json:"tokens_input,omitempty"`
	TokensOutput    *uint32  `json:"tokens_output,omitempty"`
	TokensPerSecond *float32 `json:"tokens_per_second,omitempty"`
	TtftMs          *float32 `json:"ttft_ms,omitempty"`
	CostUSD         *float64 `json:"cost_usd,omitempty"`

	Attributes map[string]string `json:"attributes,omitempty"`
	Children   []*SpanDetail     `json:"children"`
}

// TraceDetail is the full trace with nested span tree.
type TraceDetail struct {
	TraceID     string    `json:"trace_id"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	DurationMs  float64   `json:"duration_ms"`
	ServiceName string    `json:"service_name"`
	Environment string    `json:"environment"`
	SpanCount   uint64    `json:"span_count"`
	ErrorCount  uint64    `json:"error_count"`
	Spans       []*SpanDetail `json:"spans"`
}

// TraceFilter holds query parameters for trace listing.
type TraceFilter struct {
	TenantID       string
	ServiceName    *string
	Status         *string
	Environment    *string
	ModelName      *string
	MinDurationMs  *float64
	MaxDurationMs  *float64
	StartTime      *time.Time
	EndTime        *time.Time
	Limit          int
	Offset         int
}

// PhysicalGPURecord maps to the physical_gpus PostgreSQL table.
type PhysicalGPURecord struct {
	TenantID      string  `json:"-"`
	UUID          string  `json:"uuid"`
	Model         string  `json:"model"`
	Vendor        string  `json:"vendor"`
	MemoryTotalGB float32 `json:"memory_total_gb"`
	NodeID        string  `json:"node_id"`
}

// ComputeResourceRecord maps to the compute_resources PostgreSQL table.
type ComputeResourceRecord struct {
	TenantID     string  `json:"-"`
	ResourceUUID string  `json:"resource_uuid"`
	PhysicalUUID string  `json:"physical_uuid"`
	ResourceType string  `json:"resource_type"`
	MemoryGB     float32 `json:"memory_gb"`
}

// ResourceContextRecord maps to the resource_contexts PostgreSQL table.
type ResourceContextRecord struct {
	TenantID     string `json:"-"`
	ResourceUUID string `json:"resource_uuid"`
	UserLabel    string `json:"user_label"`
	Hostname     string `json:"hostname"`
}

// GPUSummary is returned by the GPU list query.
type GPUSummary struct {
	ResourceUUID  string  `json:"resource_uuid"`
	PhysicalUUID  string  `json:"physical_uuid"`
	Model         string  `json:"model"`
	Vendor        string  `json:"vendor"`
	ResourceType  string  `json:"resource_type"`
	NodeID        string  `json:"node_id"`
	Utilization   float32 `json:"utilization"`
	MemoryUsedGB  float32 `json:"memory_used_gb"`
	MemoryTotalGB float32 `json:"memory_total_gb"`
}

// GPUDetail is the full GPU detail response.
type GPUDetail struct {
	ResourceUUID string    `json:"resource_uuid"`
	PhysicalUUID string    `json:"physical_uuid"`
	Model        string    `json:"model"`
	ResourceType string    `json:"resource_type"`
	NodeID       string    `json:"node_id"`
	FirstSeen    time.Time `json:"first_seen"`
	LastSeen     time.Time `json:"last_seen"`
}

// GPUMetricRow is a single GPU metric data point.
type GPUMetricRow struct {
	Timestamp          time.Time `json:"timestamp"`
	ResourceUUID       string    `json:"resource_uuid"`
	Utilization        float32   `json:"utilization"`
	MemoryUsedGB       float32   `json:"memory_used_gb"`
	MemoryTotalGB      float32   `json:"memory_total_gb"`
	PowerWatts         uint16    `json:"power_watts"`
	TemperatureCelsius int16     `json:"temperature_celsius"`
	ClockMHz           int16     `json:"clock_mhz"`
	ActiveSpans        int16     `json:"active_spans"`
}

// AnalyticsOverview is the overview response for the dashboard.
type AnalyticsOverview struct {
	TotalTraces      int                `json:"total_traces"`
	AvgLatencyMs     float64            `json:"avg_latency_ms"`
	ErrorRate        float64            `json:"error_rate"`
	ActiveGPUCount   int                `json:"active_gpu_count"`
	TotalCostUSD     float64            `json:"total_cost_usd"`
	ThroughputSeries []ThroughputPoint  `json:"throughput_series"`
	LatencySeries    []LatencyPoint     `json:"latency_series"`
}

// ModelStats is a per-model analytics summary.
type ModelStats struct {
	ModelName        string  `json:"model_name"`
	TotalInferences  int     `json:"total_inferences"`
	AvgLatencyMs     float64 `json:"avg_latency_ms"`
	AvgTtftMs        float64 `json:"avg_ttft_ms"`
	AvgTokensPerSec  float64 `json:"avg_tokens_per_second"`
	TotalTokensIn    int64   `json:"total_tokens_input"`
	TotalTokensOut   int64   `json:"total_tokens_output"`
	TotalCostUSD     float64 `json:"total_cost_usd"`
	ErrorRate        float64 `json:"error_rate"`
}

// ServiceStats is a per-service analytics summary.
type ServiceStats struct {
	ServiceName      string  `json:"service_name"`
	TotalTraces      int     `json:"total_traces"`
	AvgLatencyMs     float64 `json:"avg_latency_ms"`
	ErrorRate        float64 `json:"error_rate"`
	ThroughputPerHr  float64 `json:"throughput_per_hour"`
}

// ErrorGroup is a grouped error summary.
type ErrorGroup struct {
	ErrorMessage string `json:"error_message"`
	Count        int    `json:"count"`
	ServiceName  string `json:"service_name"`
	ModelName    string `json:"model_name"`
	LastSeen     time.Time `json:"last_seen"`
}

// FilterOptions holds distinct values for filter dropdowns.
type FilterOptions struct {
	ServiceNames   []string `json:"service_names"`
	Environments   []string `json:"environments"`
	ModelNames     []string `json:"model_names"`
	InferenceTypes []string `json:"inference_types"`
}

// GPUFleetOverview is the aggregate GPU fleet status.
type GPUFleetOverview struct {
	TotalGPUs      int              `json:"total_gpus"`
	ByVendor       map[string]int   `json:"by_vendor"`
	ByModel        map[string]int   `json:"by_model"`
	AvgUtilization float64          `json:"avg_utilization"`
	AvgPowerWatts  float64          `json:"avg_power_watts"`
}

// ThroughputPoint is a single point in the throughput time series.
type ThroughputPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Count     int       `json:"count"`
}

// LatencyPoint is a single point in the latency percentile time series.
type LatencyPoint struct {
	Timestamp time.Time `json:"timestamp"`
	P50Ms     float64   `json:"p50_ms"`
	P95Ms     float64   `json:"p95_ms"`
	P99Ms     float64   `json:"p99_ms"`
}
