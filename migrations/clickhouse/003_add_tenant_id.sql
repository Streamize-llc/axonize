-- Multi-tenant: add tenant_id to all ClickHouse tables.
-- Existing data gets 'default' tenant.
-- ClickHouse MergeTree ORDER BY cannot be changed, so we use bloom_filter indexes.

ALTER TABLE spans ADD COLUMN IF NOT EXISTS tenant_id String DEFAULT 'default' AFTER trace_id;
ALTER TABLE spans ADD INDEX IF NOT EXISTS idx_tenant_id tenant_id TYPE bloom_filter GRANULARITY 1;

ALTER TABLE traces ADD COLUMN IF NOT EXISTS tenant_id String DEFAULT 'default' AFTER trace_id;
ALTER TABLE traces ADD INDEX IF NOT EXISTS idx_traces_tenant_id tenant_id TYPE bloom_filter GRANULARITY 1;

ALTER TABLE gpu_metrics ADD COLUMN IF NOT EXISTS tenant_id String DEFAULT 'default' AFTER timestamp;
ALTER TABLE gpu_metrics ADD INDEX IF NOT EXISTS idx_gpu_metrics_tenant_id tenant_id TYPE bloom_filter GRANULARITY 1;
