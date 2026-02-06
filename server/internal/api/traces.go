package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/axonize/server/internal/store"
)

// TraceQuerier is the interface for trace queries.
type TraceQuerier interface {
	QueryTraces(ctx context.Context, f store.TraceFilter) ([]store.TraceSummary, int, error)
	QueryTraceByID(ctx context.Context, traceID string) (*store.TraceDetail, error)
}

func handleListTraces(querier TraceQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		filter := store.TraceFilter{
			Limit:  50,
			Offset: 0,
		}

		if v := q.Get("service_name"); v != "" {
			filter.ServiceName = &v
		}
		if v := q.Get("start"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				filter.StartTime = &t
			}
		}
		if v := q.Get("end"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				filter.EndTime = &t
			}
		}
		if v := q.Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
				filter.Limit = n
			}
		}
		if v := q.Get("offset"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 {
				filter.Offset = n
			}
		}

		traces, total, err := querier.QueryTraces(r.Context(), filter)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if traces == nil {
			traces = []store.TraceSummary{}
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"traces": traces,
			"total":  total,
			"limit":  filter.Limit,
			"offset": filter.Offset,
		})
	}
}

func handleGetTrace(querier TraceQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		traceID := r.PathValue("trace_id")
		if traceID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "trace_id is required"})
			return
		}

		detail, err := querier.QueryTraceByID(r.Context(), traceID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if detail == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "trace not found"})
			return
		}

		writeJSON(w, http.StatusOK, detail)
	}
}
