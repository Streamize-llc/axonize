-- Multi-tenant: tenant registry, API keys, usage metering, and tenant_id on GPU tables.

-- Tenant registry
CREATE TABLE IF NOT EXISTS tenants (
    tenant_id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(256) NOT NULL,
    plan VARCHAR(32) NOT NULL DEFAULT 'free',        -- free, pro, enterprise, self-hosted
    status VARCHAR(16) NOT NULL DEFAULT 'active',    -- active, suspended, deleted
    max_spans_per_day BIGINT DEFAULT 100000,
    gpu_profiling_enabled BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

INSERT INTO tenants (tenant_id, name, plan, status)
VALUES ('default', 'Default Tenant', 'self-hosted', 'active')
ON CONFLICT (tenant_id) DO NOTHING;

-- API keys (multiple per tenant)
CREATE TABLE IF NOT EXISTS api_keys (
    key_hash VARCHAR(64) PRIMARY KEY,        -- SHA-256
    key_prefix VARCHAR(12) NOT NULL,         -- display prefix "ax_live_abc..."
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(tenant_id),
    name VARCHAR(256),
    scopes VARCHAR(256) DEFAULT 'ingest,read',
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    last_used_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_api_keys_tenant_id ON api_keys (tenant_id);

-- Usage metering (hybrid billing: spans + GPU seconds)
CREATE TABLE IF NOT EXISTS usage_records (
    id BIGSERIAL PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(tenant_id),
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    span_count BIGINT DEFAULT 0,
    gpu_seconds BIGINT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(tenant_id, period_start)
);
CREATE INDEX IF NOT EXISTS idx_usage_tenant_period ON usage_records (tenant_id, period_start);

-- Add tenant_id to existing GPU tables and rebuild PKs for composite keys.
-- Drop existing constraints first (order matters: children before parents).
ALTER TABLE resource_contexts DROP CONSTRAINT IF EXISTS resource_contexts_resource_uuid_fkey;
ALTER TABLE compute_resources DROP CONSTRAINT IF EXISTS compute_resources_physical_gpu_uuid_fkey;
ALTER TABLE resource_contexts DROP CONSTRAINT IF EXISTS resource_contexts_pkey;
ALTER TABLE compute_resources DROP CONSTRAINT IF EXISTS compute_resources_pkey;
ALTER TABLE physical_gpus DROP CONSTRAINT IF EXISTS physical_gpus_pkey;

ALTER TABLE physical_gpus ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(64) DEFAULT 'default';
ALTER TABLE compute_resources ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(64) DEFAULT 'default';
ALTER TABLE resource_contexts ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(64) DEFAULT 'default';

UPDATE physical_gpus SET tenant_id = 'default' WHERE tenant_id IS NULL;
UPDATE compute_resources SET tenant_id = 'default' WHERE tenant_id IS NULL;
UPDATE resource_contexts SET tenant_id = 'default' WHERE tenant_id IS NULL;

ALTER TABLE physical_gpus ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE compute_resources ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE resource_contexts ALTER COLUMN tenant_id SET NOT NULL;

ALTER TABLE physical_gpus ADD PRIMARY KEY (tenant_id, uuid);
ALTER TABLE compute_resources ADD PRIMARY KEY (tenant_id, resource_uuid);
ALTER TABLE resource_contexts ADD PRIMARY KEY (tenant_id, context_id);

ALTER TABLE compute_resources ADD CONSTRAINT compute_resources_physical_gpu_fk
    FOREIGN KEY (tenant_id, physical_gpu_uuid) REFERENCES physical_gpus(tenant_id, uuid);
ALTER TABLE resource_contexts ADD CONSTRAINT resource_contexts_compute_resource_fk
    FOREIGN KEY (tenant_id, resource_uuid) REFERENCES compute_resources(tenant_id, resource_uuid);
