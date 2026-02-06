-- Axonize PostgreSQL Schema (GPU Registry)

-- ============================================
-- Physical GPUs (immutable hardware info)
-- ============================================
CREATE TABLE IF NOT EXISTS physical_gpus (
    uuid VARCHAR(64) PRIMARY KEY,

    -- Hardware specs
    model VARCHAR(128) NOT NULL,
    vendor VARCHAR(32) NOT NULL,
    architecture VARCHAR(32),
    compute_capability VARCHAR(8),

    -- Memory
    memory_total_gb DECIMAL(5,1) NOT NULL,
    memory_bandwidth_gbps INTEGER,

    -- Compute
    sm_count INTEGER,
    fp16_tflops DECIMAL(6,1),
    fp32_tflops DECIMAL(6,1),
    tdp_watts INTEGER,

    -- Location
    node_id VARCHAR(128) NOT NULL,
    pcie_bus_id VARCHAR(16),
    numa_node SMALLINT,

    -- Runtime
    driver_version VARCHAR(32),
    cuda_version VARCHAR(16),

    -- Cloud
    cloud_provider VARCHAR(16),
    cloud_instance_id VARCHAR(64),
    cloud_zone VARCHAR(32),

    -- Cost
    cost_per_hour_usd DECIMAL(10,4),

    -- Meta
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    last_seen_at TIMESTAMP
);


-- ============================================
-- Compute Resources (logical compute units)
-- ============================================
CREATE TABLE IF NOT EXISTS compute_resources (
    resource_uuid VARCHAR(64) PRIMARY KEY,

    -- Type
    resource_type VARCHAR(32) NOT NULL,

    -- Physical GPU reference
    physical_gpu_uuid VARCHAR(64) NOT NULL REFERENCES physical_gpus(uuid),

    -- Specs
    memory_gb DECIMAL(5,1) NOT NULL,
    sm_count INTEGER,

    -- MIG info
    mig_profile VARCHAR(16),
    gi_id SMALLINT,
    ci_id SMALLINT,

    -- Sharing
    is_shared BOOLEAN DEFAULT FALSE,

    -- Meta
    created_at TIMESTAMP DEFAULT NOW(),
    last_seen_at TIMESTAMP
);


-- ============================================
-- Resource Contexts (runtime mapping)
-- ============================================
CREATE TABLE IF NOT EXISTS resource_contexts (
    context_id VARCHAR(64) PRIMARY KEY,

    -- Resource reference
    resource_uuid VARCHAR(64) NOT NULL REFERENCES compute_resources(resource_uuid),

    -- User perspective
    user_label VARCHAR(16) NOT NULL,

    -- Process info
    hostname VARCHAR(128) NOT NULL,
    process_id INTEGER,
    process_name VARCHAR(64),

    -- Environment metadata (generic)
    labels JSONB DEFAULT '{}',

    -- Timing
    attached_at TIMESTAMP NOT NULL,
    detached_at TIMESTAMP
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_resource_contexts_resource_uuid ON resource_contexts (resource_uuid);
CREATE INDEX IF NOT EXISTS idx_resource_contexts_hostname ON resource_contexts (hostname);
CREATE INDEX IF NOT EXISTS idx_resource_contexts_labels ON resource_contexts USING GIN (labels);
