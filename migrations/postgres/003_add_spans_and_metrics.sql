-- Spans and GPU Metrics tables (migrated from ClickHouse to PostgreSQL)

-- ============================================
-- Spans table (main time-series data)
-- ============================================
CREATE TABLE IF NOT EXISTS spans (
    tenant_id VARCHAR(64) NOT NULL,
    trace_id VARCHAR(64) NOT NULL,
    span_id VARCHAR(32) NOT NULL,
    parent_span_id VARCHAR(32),

    -- Basic info
    name VARCHAR(256) NOT NULL,
    service_name VARCHAR(128) NOT NULL,
    environment VARCHAR(64) NOT NULL DEFAULT '',

    -- Timing
    start_time TIMESTAMPTZ(3) NOT NULL,
    end_time TIMESTAMPTZ(3) NOT NULL,
    duration_ms DOUBLE PRECISION NOT NULL,

    -- AI model info
    model_name VARCHAR(128),
    model_version VARCHAR(32),
    inference_type VARCHAR(32),

    -- LLM metrics
    tokens_input INTEGER,
    tokens_output INTEGER,
    tokens_per_second REAL,
    ttft_ms REAL,

    -- Diffusion metrics
    diffusion_steps SMALLINT,
    cfg_scale REAL,

    -- GPU info (denormalized arrays)
    gpu_resource_uuids TEXT[] DEFAULT '{}',
    gpu_physical_uuids TEXT[] DEFAULT '{}',
    gpu_models TEXT[] DEFAULT '{}',
    gpu_vendors TEXT[] DEFAULT '{}',
    gpu_node_ids TEXT[] DEFAULT '{}',
    gpu_memory_used_gb REAL[] DEFAULT '{}',
    gpu_utilization REAL[] DEFAULT '{}',
    gpu_power_watts INTEGER[] DEFAULT '{}',

    -- Cost
    cost_usd DOUBLE PRECISION,

    -- Status
    status VARCHAR(16) NOT NULL DEFAULT 'ok',
    error_message TEXT,

    -- Attributes (flexible extension)
    attributes JSONB DEFAULT '{}',

    PRIMARY KEY (tenant_id, trace_id, span_id)
);

CREATE INDEX IF NOT EXISTS idx_spans_tenant_time ON spans (tenant_id, start_time DESC);
CREATE INDEX IF NOT EXISTS idx_spans_trace_id ON spans (trace_id);
CREATE INDEX IF NOT EXISTS idx_spans_service ON spans (tenant_id, service_name, start_time DESC);

-- ============================================
-- GPU Metrics table (time-series GPU state)
-- ============================================
CREATE TABLE IF NOT EXISTS gpu_metrics (
    tenant_id VARCHAR(64) NOT NULL,
    timestamp TIMESTAMPTZ(3) NOT NULL,
    resource_uuid VARCHAR(64) NOT NULL,
    physical_gpu_uuid VARCHAR(64) NOT NULL,
    node_id VARCHAR(128) NOT NULL,

    -- State
    utilization REAL NOT NULL DEFAULT 0,
    memory_used_gb REAL NOT NULL DEFAULT 0,
    memory_total_gb REAL NOT NULL DEFAULT 0,
    temperature_celsius SMALLINT NOT NULL DEFAULT 0,
    power_watts SMALLINT NOT NULL DEFAULT 0,
    clock_mhz SMALLINT NOT NULL DEFAULT 0,

    -- Inference activity
    active_spans SMALLINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_gpu_metrics_lookup ON gpu_metrics (tenant_id, resource_uuid, timestamp DESC);
