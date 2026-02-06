package api

import "net/http"

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
func NewRouter(s Store, gpuStore GPUStore) http.Handler {
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

	return corsMiddleware(mux)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
