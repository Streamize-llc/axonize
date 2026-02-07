package tenant

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const cacheTTL = 5 * time.Minute

type cacheEntry struct {
	tenantID  string
	expiresAt time.Time
}

// Resolver resolves a raw API key to a tenant_id, with an in-memory cache.
type Resolver struct {
	pool   *pgxpool.Pool
	logger *slog.Logger

	mu    sync.RWMutex
	cache map[string]cacheEntry
}

// NewResolver creates a new tenant resolver backed by the api_keys table.
func NewResolver(pool *pgxpool.Pool, logger *slog.Logger) *Resolver {
	return &Resolver{
		pool:   pool,
		logger: logger,
		cache:  make(map[string]cacheEntry),
	}
}

// Resolve maps a raw API key to its tenant_id.
// It hashes the key with SHA-256, looks up the api_keys table,
// and caches the result for 5 minutes.
func (r *Resolver) Resolve(ctx context.Context, rawKey string) (string, error) {
	hash := hashKey(rawKey)

	// Check cache
	r.mu.RLock()
	if entry, ok := r.cache[hash]; ok && time.Now().Before(entry.expiresAt) {
		r.mu.RUnlock()
		return entry.tenantID, nil
	}
	r.mu.RUnlock()

	// DB lookup
	var tenantID, keyStatus string
	err := r.pool.QueryRow(ctx, `
		SELECT ak.tenant_id, ak.status
		FROM api_keys ak
		JOIN tenants t ON t.tenant_id = ak.tenant_id
		WHERE ak.key_hash = $1
		  AND t.status = 'active'
	`, hash).Scan(&tenantID, &keyStatus)
	if err != nil {
		return "", fmt.Errorf("resolve api key: %w", err)
	}

	if keyStatus != "active" {
		return "", fmt.Errorf("api key is %s", keyStatus)
	}

	// Cache the result
	r.mu.Lock()
	r.cache[hash] = cacheEntry{
		tenantID:  tenantID,
		expiresAt: time.Now().Add(cacheTTL),
	}
	r.mu.Unlock()

	// Async update last_used_at (fire and forget)
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, err := r.pool.Exec(bgCtx, `UPDATE api_keys SET last_used_at = NOW() WHERE key_hash = $1`, hash)
		if err != nil {
			r.logger.Debug("failed to update last_used_at", "error", err)
		}
	}()

	return tenantID, nil
}

// Pool exposes the underlying connection pool for admin operations.
func (r *Resolver) Pool() *pgxpool.Pool {
	return r.pool
}

func hashKey(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
