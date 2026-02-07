package api

import (
	"net/http"
	"strings"

	"github.com/axonize/server/internal/tenant"
)

// Store combines all query interfaces needed by the API (ClickHouse).
type Store interface {
	Pinger
	TraceQuerier
	GPUMetricQuerier
	AnalyticsQuerier
}

// GPUStore combines GPU registry query interfaces (PostgreSQL).
type GPUStore interface {
	GPUQuerier
}

// NewRouter creates the HTTP router with all API endpoints.
func NewRouter(s Store, gpuStore GPUStore, apiKey, authMode string, resolver *tenant.Resolver, adminKey string) http.Handler {
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /readyz", handleReadyz(s))

	// Traces
	mux.HandleFunc("GET /api/v1/traces", handleListTraces(s))
	mux.HandleFunc("GET /api/v1/traces/{trace_id}", handleGetTrace(s))

	// GPUs
	mux.HandleFunc("GET /api/v1/gpus", handleListGPUs(gpuStore))
	mux.HandleFunc("GET /api/v1/gpus/{uuid}", handleGetGPU(gpuStore))
	mux.HandleFunc("GET /api/v1/gpus/{uuid}/metrics", handleGetGPUMetrics(s))

	// Analytics
	mux.HandleFunc("GET /api/v1/analytics/overview", handleAnalyticsOverview(s))

	// Admin routes (multi-tenant only)
	if authMode == "multi_tenant" && adminKey != "" {
		adminMux := http.NewServeMux()
		registerAdminRoutes(adminMux, resolver)
		mux.Handle("/api/v1/admin/", adminKeyMiddleware(adminMux, adminKey))
	}

	var handler http.Handler = mux
	if authMode == "multi_tenant" && resolver != nil {
		handler = multiTenantMiddleware(handler, resolver)
	} else if apiKey != "" {
		handler = apiKeyMiddleware(handler, apiKey)
	}
	return corsMiddleware(handler)
}

// apiKeyMiddleware validates the Authorization: Bearer <key> header.
// Health endpoints (/healthz, /readyz) are exempt.
// Sets tenant_id to "default" for static auth mode.
func apiKeyMiddleware(next http.Handler, apiKey string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}

		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || auth[7:] != apiKey {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := tenant.WithTenantID(r.Context(), tenant.DefaultTenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// multiTenantMiddleware resolves the API key to a tenant_id via the Resolver.
// Health endpoints (/healthz, /readyz) are exempt.
func multiTenantMiddleware(next http.Handler, resolver *tenant.Resolver) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}

		// Admin routes use their own auth middleware
		if strings.HasPrefix(r.URL.Path, "/api/v1/admin/") {
			next.ServeHTTP(w, r)
			return
		}

		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		rawKey := auth[7:]

		tenantID, err := resolver.Resolve(r.Context(), rawKey)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := tenant.WithTenantID(r.Context(), tenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// adminKeyMiddleware validates the admin key for admin endpoints.
func adminKeyMiddleware(next http.Handler, adminKey string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || auth[7:] != adminKey {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
