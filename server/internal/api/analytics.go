package api

import (
	"context"
	"net/http"
	"time"

	"github.com/axonize/server/internal/store"
)

// AnalyticsQuerier is the interface for analytics queries.
type AnalyticsQuerier interface {
	QueryAnalyticsOverview(ctx context.Context, start, end time.Time) (*store.AnalyticsOverview, error)
}

func handleAnalyticsOverview(querier AnalyticsQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

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

		overview, err := querier.QueryAnalyticsOverview(r.Context(), start, end)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, overview)
	}
}
