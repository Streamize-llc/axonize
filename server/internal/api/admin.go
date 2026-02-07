package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/axonize/server/internal/tenant"
)

// registerAdminRoutes adds admin endpoints to the given mux.
// These are protected by adminKeyMiddleware in the router.
func registerAdminRoutes(mux *http.ServeMux, resolver *tenant.Resolver) {
	pool := resolver.Pool()

	mux.HandleFunc("POST /api/v1/admin/tenants", handleCreateTenant(pool))
	mux.HandleFunc("GET /api/v1/admin/tenants", handleListTenants(pool))
	mux.HandleFunc("POST /api/v1/admin/tenants/{id}/keys", handleCreateAPIKey(pool))
	mux.HandleFunc("DELETE /api/v1/admin/tenants/{id}/keys/{prefix}", handleRevokeAPIKey(pool))
	mux.HandleFunc("GET /api/v1/admin/tenants/{id}/usage", handleGetUsage(pool))
}

type createTenantRequest struct {
	Name string `json:"name"`
	Plan string `json:"plan"`
}

type createTenantResponse struct {
	TenantID  string `json:"tenant_id"`
	Name      string `json:"name"`
	Plan      string `json:"plan"`
	CreatedAt string `json:"created_at"`
}

func handleCreateTenant(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createTenantRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if req.Name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
			return
		}
		if req.Plan == "" {
			req.Plan = "free"
		}

		tenantID := generateTenantID()
		now := time.Now()

		_, err := pool.Exec(r.Context(), `
			INSERT INTO tenants (tenant_id, name, plan, status, created_at, updated_at)
			VALUES ($1, $2, $3, 'active', $4, $4)
		`, tenantID, req.Name, req.Plan, now)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("create tenant: %v", err)})
			return
		}

		writeJSON(w, http.StatusCreated, createTenantResponse{
			TenantID:  tenantID,
			Name:      req.Name,
			Plan:      req.Plan,
			CreatedAt: now.Format(time.RFC3339),
		})
	}
}

type tenantListItem struct {
	TenantID  string `json:"tenant_id"`
	Name      string `json:"name"`
	Plan      string `json:"plan"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

func handleListTenants(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := pool.Query(r.Context(), `
			SELECT tenant_id, name, plan, status, created_at
			FROM tenants
			ORDER BY created_at DESC
		`)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		defer rows.Close()

		var tenants []tenantListItem
		for rows.Next() {
			var t tenantListItem
			var createdAt time.Time
			if err := rows.Scan(&t.TenantID, &t.Name, &t.Plan, &t.Status, &createdAt); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			t.CreatedAt = createdAt.Format(time.RFC3339)
			tenants = append(tenants, t)
		}
		if tenants == nil {
			tenants = []tenantListItem{}
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"tenants": tenants,
		})
	}
}

type createAPIKeyRequest struct {
	Name   string `json:"name"`
	Scopes string `json:"scopes"`
}

type createAPIKeyResponse struct {
	Key       string `json:"key"`
	KeyPrefix string `json:"key_prefix"`
	Name      string `json:"name"`
	Scopes    string `json:"scopes"`
	CreatedAt string `json:"created_at"`
}

func handleCreateAPIKey(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		if tenantID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tenant id is required"})
			return
		}

		// Verify tenant exists
		var status string
		err := pool.QueryRow(r.Context(), `SELECT status FROM tenants WHERE tenant_id = $1`, tenantID).Scan(&status)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "tenant not found"})
			return
		}
		if status != "active" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("tenant is %s", status)})
			return
		}

		var req createAPIKeyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if req.Scopes == "" {
			req.Scopes = "ingest,read"
		}

		// Generate raw key
		rawKey := generateAPIKey()
		keyPrefix := rawKey[:12]
		keyHash := hashKey(rawKey)
		now := time.Now()

		_, err = pool.Exec(r.Context(), `
			INSERT INTO api_keys (key_hash, key_prefix, tenant_id, name, scopes, status, created_at)
			VALUES ($1, $2, $3, $4, $5, 'active', $6)
		`, keyHash, keyPrefix, tenantID, req.Name, req.Scopes, now)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("create api key: %v", err)})
			return
		}

		writeJSON(w, http.StatusCreated, createAPIKeyResponse{
			Key:       rawKey,
			KeyPrefix: keyPrefix,
			Name:      req.Name,
			Scopes:    req.Scopes,
			CreatedAt: now.Format(time.RFC3339),
		})
	}
}

func handleRevokeAPIKey(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		prefix := r.PathValue("prefix")
		if tenantID == "" || prefix == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tenant id and key prefix are required"})
			return
		}

		result, err := pool.Exec(r.Context(), `
			UPDATE api_keys SET status = 'revoked'
			WHERE tenant_id = $1 AND key_prefix = $2 AND status = 'active'
		`, tenantID, prefix)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if result.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "api key not found or already revoked"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
	}
}

type usageResponse struct {
	TenantID   string `json:"tenant_id"`
	Date       string `json:"date"`
	SpanCount  int64  `json:"span_count"`
	GPUSeconds int64  `json:"gpu_seconds"`
}

func handleGetUsage(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.PathValue("id")
		if tenantID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tenant id is required"})
			return
		}

		today := time.Now().UTC().Truncate(24 * time.Hour)
		var spans, gpuSec int64

		err := pool.QueryRow(r.Context(), `
			SELECT COALESCE(span_count, 0), COALESCE(gpu_seconds, 0)
			FROM usage_records
			WHERE tenant_id = $1 AND period_start = $2
		`, tenantID, today).Scan(&spans, &gpuSec)
		if err != nil {
			// No usage record yet
			spans = 0
			gpuSec = 0
		}

		writeJSON(w, http.StatusOK, usageResponse{
			TenantID:   tenantID,
			Date:       today.Format("2006-01-02"),
			SpanCount:  spans,
			GPUSeconds: gpuSec,
		})
	}
}

// generateTenantID creates a random tenant ID with "tn_" prefix.
func generateTenantID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return "tn_" + hex.EncodeToString(b)
}

// generateAPIKey creates a random API key with "ax_live_" prefix.
func generateAPIKey() string {
	b := make([]byte, 24)
	rand.Read(b)
	return "ax_live_" + hex.EncodeToString(b)
}

func hashKey(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

