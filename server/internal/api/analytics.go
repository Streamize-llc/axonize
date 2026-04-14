package api

import (
	"context"
	"net/http"
	"time"

	"github.com/axonize/server/internal/store"
	"github.com/axonize/server/internal/tenant"
)

// AnalyticsQuerier is the interface for analytics queries.
type AnalyticsQuerier interface {
	QueryAnalyticsOverview(ctx context.Context, tenantID string, start, end time.Time) (*store.AnalyticsOverview, error)
	QueryModelStats(ctx context.Context, tenantID string, start, end time.Time) ([]store.ModelStats, error)
	QueryServiceStats(ctx context.Context, tenantID string, start, end time.Time) ([]store.ServiceStats, error)
	QueryErrorGroups(ctx context.Context, tenantID string, start, end time.Time) ([]store.ErrorGroup, error)
	QueryFilterOptions(ctx context.Context, tenantID string) (*store.FilterOptions, error)
	QueryGPUFleet(ctx context.Context, tenantID string) (*store.GPUFleetOverview, error)
}

func handleAnalyticsOverview(querier AnalyticsQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		tenantID := tenant.FromContext(r.Context())

		end := time.Now()
		start := end.Add(-24 * time.Hour)

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

		overview, err := querier.QueryAnalyticsOverview(r.Context(), tenantID, start, end)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, overview)
	}
}

func handleAnalyticsModels(querier AnalyticsQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		tenantID := tenant.FromContext(r.Context())
		end := time.Now()
		start := end.Add(-24 * time.Hour)
		if v := q.Get("start"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil { start = t }
		}
		if v := q.Get("end"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil { end = t }
		}

		models, err := querier.QueryModelStats(r.Context(), tenantID, start, end)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"models": models})
	}
}

func handleAnalyticsServices(querier AnalyticsQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		tenantID := tenant.FromContext(r.Context())
		end := time.Now()
		start := end.Add(-24 * time.Hour)
		if v := q.Get("start"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil { start = t }
		}
		if v := q.Get("end"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil { end = t }
		}

		services, err := querier.QueryServiceStats(r.Context(), tenantID, start, end)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"services": services})
	}
}

func handleAnalyticsErrors(querier AnalyticsQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		tenantID := tenant.FromContext(r.Context())
		end := time.Now()
		start := end.Add(-24 * time.Hour)
		if v := q.Get("start"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil { start = t }
		}
		if v := q.Get("end"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil { end = t }
		}

		errors, err := querier.QueryErrorGroups(r.Context(), tenantID, start, end)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"errors": errors})
	}
}

func handleMetaFilters(querier AnalyticsQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.FromContext(r.Context())
		opts, err := querier.QueryFilterOptions(r.Context(), tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, opts)
	}
}

func handleGPUFleet(querier AnalyticsQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenant.FromContext(r.Context())
		fleet, err := querier.QueryGPUFleet(r.Context(), tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, fleet)
	}
}
