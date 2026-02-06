-- Axonize ClickHouse Schema
-- Spans, Traces, GPU Metrics tables

-- ============================================
-- Spans table (main time-series data)
-- ============================================
CREATE TABLE IF NOT EXISTS spans (
    -- Identifiers
    trace_id String,
    span_id String,
    parent_span_id Nullable(String),

    -- Basic info
    name String,
    service_name String,
    environment String,

    -- Timing
    start_time DateTime64(3),
    end_time DateTime64(3),
    duration_ms Float64,

    -- AI model info
    model_name Nullable(String),
    model_version Nullable(String),
    inference_type Nullable(String),

    -- LLM metrics
    tokens_input Nullable(UInt32),
    tokens_output Nullable(UInt32),
    tokens_per_second Nullable(Float32),
    ttft_ms Nullable(Float32),

    -- Diffusion metrics
    diffusion_steps Nullable(UInt16),
    cfg_scale Nullable(Float32),

    -- GPU info (denormalized)
    gpu_resource_uuids Array(String),
    gpu_physical_uuids Array(String),
    gpu_models Array(String),
    gpu_node_ids Array(String),
    gpu_memory_used_gb Array(Float32),
    gpu_utilization Array(Float32),
    gpu_power_watts Array(UInt16),

    -- Cost
    cost_usd Nullable(Float64),

    -- Status
    status String,
    error_message Nullable(String),

    -- Attributes (flexible extension)
    attributes Map(String, String),

    -- Indexes
    INDEX idx_trace_id trace_id TYPE bloom_filter GRANULARITY 1,
    INDEX idx_model_name model_name TYPE bloom_filter GRANULARITY 1,
    INDEX idx_service_name service_name TYPE bloom_filter GRANULARITY 1
)
ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(start_time)
ORDER BY (service_name, start_time, trace_id)
TTL start_time + INTERVAL 30 DAY;


-- ============================================
-- Traces table (aggregated trace info)
-- ============================================
CREATE TABLE IF NOT EXISTS traces (
    trace_id String,

    -- Timing
    start_time DateTime64(3),
    end_time DateTime64(3),
    duration_ms Float64,

    -- Metadata
    service_name String,
    environment String,
    root_span_name String,

    -- Aggregation
    span_count UInt32,
    error_count UInt32,

    -- Cost
    total_cost_usd Float64,

    -- GPU usage summary
    gpu_count UInt8,
    total_gpu_time_ms Float64,

    INDEX idx_trace_id trace_id TYPE bloom_filter GRANULARITY 1
)
ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(start_time)
ORDER BY (service_name, start_time)
TTL start_time + INTERVAL 90 DAY;


-- ============================================
-- GPU Metrics table (time-series GPU state)
-- ============================================
CREATE TABLE IF NOT EXISTS gpu_metrics (
    timestamp DateTime64(3),

    -- GPU identification
    resource_uuid String,
    physical_gpu_uuid String,
    node_id String,

    -- State
    utilization Float32,
    memory_used_gb Float32,
    memory_total_gb Float32,
    temperature_celsius UInt8,
    power_watts UInt16,
    clock_mhz UInt16,

    -- Inference activity
    active_spans UInt16,

    INDEX idx_resource_uuid resource_uuid TYPE bloom_filter GRANULARITY 1
)
ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (node_id, resource_uuid, timestamp)
TTL timestamp + INTERVAL 7 DAY;
