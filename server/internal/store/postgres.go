package store

import (
	"context"
	"fmt"
	"log/slog"

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
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database,
	)

	pool, err := pgxpool.New(context.Background(), dsn)
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
		return nil, fmt.Errorf("get gpu: %w", err)
	}
	return &g, nil
}

// Ensure PostgresStore satisfies compile-time interfaces.
var _ interface {
	UpsertPhysicalGPU(ctx context.Context, gpu PhysicalGPURecord) error
	UpsertComputeResource(ctx context.Context, res ComputeResourceRecord) error
	UpsertResourceContext(ctx context.Context, rc ResourceContextRecord) error
} = (*PostgresStore)(nil)
