package api

import (
	"context"
	"net/http"
	"time"

	"github.com/axonize/server/internal/store"
)

// GPUQuerier is the interface for GPU queries.
type GPUQuerier interface {
	ListGPUs(ctx context.Context) ([]store.GPUSummary, error)
	GetGPU(ctx context.Context, uuid string) (*store.GPUDetail, error)
}

// GPUMetricQuerier is the interface for GPU metric time-series queries.
type GPUMetricQuerier interface {
	QueryGPUMetrics(ctx context.Context, uuid string, start, end time.Time) ([]store.GPUMetricRow, error)
}

func handleListGPUs(querier GPUQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gpus, err := querier.ListGPUs(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if gpus == nil {
			gpus = []store.GPUSummary{}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"gpus": gpus,
		})
	}
}

func handleGetGPU(querier GPUQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		if uuid == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "uuid is required"})
			return
		}

		detail, err := querier.GetGPU(r.Context(), uuid)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "gpu not found"})
			return
		}

		writeJSON(w, http.StatusOK, detail)
	}
}

func handleGetGPUMetrics(querier GPUMetricQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		if uuid == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "uuid is required"})
			return
		}

		q := r.URL.Query()

		// Default to last 1 hour
		end := time.Now()
		start := end.Add(-1 * time.Hour)

		if v := q.Get("start"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				start = t
			}
		}
		if v := q.Get("end"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				end = t
			}
		}

		metrics, err := querier.QueryGPUMetrics(r.Context(), uuid, start, end)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if metrics == nil {
			metrics = []store.GPUMetricRow{}
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"metrics": metrics,
		})
	}
}
