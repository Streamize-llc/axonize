package store

import "time"

// SpanRecord maps 1:1 to the ClickHouse spans table.
type SpanRecord struct {
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
	Attributes   map[string]string `json:"attributes,omitempty"`
	Children     []*SpanDetail     `json:"children"`
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
	ServiceName *string
	StartTime   *time.Time
	EndTime     *time.Time
	Limit       int
	Offset      int
}

// PhysicalGPURecord maps to the physical_gpus PostgreSQL table.
type PhysicalGPURecord struct {
	UUID          string  `json:"uuid"`
	Model         string  `json:"model"`
	Vendor        string  `json:"vendor"`
	MemoryTotalGB float32 `json:"memory_total_gb"`
	NodeID        string  `json:"node_id"`
}

// ComputeResourceRecord maps to the compute_resources PostgreSQL table.
type ComputeResourceRecord struct {
	ResourceUUID string  `json:"resource_uuid"`
	PhysicalUUID string  `json:"physical_uuid"`
	ResourceType string  `json:"resource_type"`
	MemoryGB     float32 `json:"memory_gb"`
}

// ResourceContextRecord maps to the resource_contexts PostgreSQL table.
type ResourceContextRecord struct {
	ResourceUUID string `json:"resource_uuid"`
	UserLabel    string `json:"user_label"`
	Hostname     string `json:"hostname"`
}

// GPUSummary is returned by the GPU list query.
type GPUSummary struct {
	ResourceUUID string  `json:"resource_uuid"`
	PhysicalUUID string  `json:"physical_uuid"`
	Model        string  `json:"model"`
	ResourceType string  `json:"resource_type"`
	NodeID       string  `json:"node_id"`
	Utilization  float32 `json:"utilization"`
	MemoryUsedGB float32 `json:"memory_used_gb"`
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

// GPUMetricRow is a single GPU metric data point from ClickHouse.
type GPUMetricRow struct {
	Timestamp    time.Time `json:"timestamp"`
	ResourceUUID string    `json:"resource_uuid"`
	Utilization  float32   `json:"utilization"`
	MemoryUsedGB float32   `json:"memory_used_gb"`
	PowerWatts   uint16    `json:"power_watts"`
}
