package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/axonize/server/internal/store"
	"github.com/axonize/server/internal/tenant"
)

// --- Mocks ---

type mockStore struct {
	traces         []store.TraceSummary
	traceTotal     int
	traceDetail    *store.TraceDetail
	gpuMetrics     []store.GPUMetricRow
	overview       *store.AnalyticsOverview
	pingErr        error
	tracesErr      error
	traceDetailErr error
	gpuMetricsErr  error
	overviewErr    error
}

func (m *mockStore) Ping(_ context.Context) error {
	return m.pingErr
}

func (m *mockStore) QueryTraces(_ context.Context, _ store.TraceFilter) ([]store.TraceSummary, int, error) {
	return m.traces, m.traceTotal, m.tracesErr
}

func (m *mockStore) QueryTraceByID(_ context.Context, _, _ string) (*store.TraceDetail, error) {
	return m.traceDetail, m.traceDetailErr
}

func (m *mockStore) QueryGPUMetrics(_ context.Context, _, _ string, _, _ time.Time) ([]store.GPUMetricRow, error) {
	return m.gpuMetrics, m.gpuMetricsErr
}

func (m *mockStore) QueryAnalyticsOverview(_ context.Context, _ string, _, _ time.Time) (*store.AnalyticsOverview, error) {
	return m.overview, m.overviewErr
}

type mockGPUStore struct {
	gpus    []store.GPUSummary
	detail  *store.GPUDetail
	listErr error
	getErr  error
}

func (m *mockGPUStore) ListGPUs(_ context.Context, _ string) ([]store.GPUSummary, error) {
	return m.gpus, m.listErr
}

func (m *mockGPUStore) GetGPU(_ context.Context, _, _ string) (*store.GPUDetail, error) {
	return m.detail, m.getErr
}

// --- Helpers ---

func withTenant(r *http.Request, id string) *http.Request {
	ctx := tenant.WithTenantID(r.Context(), id)
	return r.WithContext(ctx)
}

func parseJSONResponse(t *testing.T, w *httptest.ResponseRecorder, v interface{}) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), v); err != nil {
		t.Fatalf("parse response: %v\nbody: %s", err, w.Body.String())
	}
}

// --- Tests ---

func TestHandleHealthz(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/healthz", nil)
	handleHealthz(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var body map[string]string
	parseJSONResponse(t, w, &body)
	if body["status"] != "ok" {
		t.Errorf("status = %q, want %q", body["status"], "ok")
	}
}

func TestHandleReadyz_Healthy(t *testing.T) {
	s := &mockStore{}
	handler := handleReadyz(s)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/readyz", nil)
	handler(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandleReadyz_Unhealthy(t *testing.T) {
	s := &mockStore{pingErr: fmt.Errorf("connection refused")}
	handler := handleReadyz(s)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/readyz", nil)
	handler(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestHandleListTraces(t *testing.T) {
	now := time.Now()
	s := &mockStore{
		traces: []store.TraceSummary{
			{TraceID: "abc123", ServiceName: "my-svc", DurationMs: 150, StartTime: now, EndTime: now.Add(150 * time.Millisecond)},
		},
		traceTotal: 1,
	}

	handler := handleListTraces(s)
	w := httptest.NewRecorder()
	r := withTenant(httptest.NewRequest("GET", "/api/v1/traces?limit=10&offset=0", nil), "default")
	handler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var body map[string]interface{}
	parseJSONResponse(t, w, &body)
	traces := body["traces"].([]interface{})
	if len(traces) != 1 {
		t.Errorf("traces count = %d, want 1", len(traces))
	}
	total := int(body["total"].(float64))
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
}

func TestHandleListTraces_Empty(t *testing.T) {
	s := &mockStore{traces: nil, traceTotal: 0}
	handler := handleListTraces(s)

	w := httptest.NewRecorder()
	r := withTenant(httptest.NewRequest("GET", "/api/v1/traces", nil), "default")
	handler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var body map[string]interface{}
	parseJSONResponse(t, w, &body)
	traces := body["traces"].([]interface{})
	if len(traces) != 0 {
		t.Errorf("traces count = %d, want 0", len(traces))
	}
}

func TestHandleListTraces_Error(t *testing.T) {
	s := &mockStore{tracesErr: fmt.Errorf("db error")}
	handler := handleListTraces(s)

	w := httptest.NewRecorder()
	r := withTenant(httptest.NewRequest("GET", "/api/v1/traces", nil), "default")
	handler(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestHandleGetTrace_Found(t *testing.T) {
	now := time.Now()
	s := &mockStore{
		traceDetail: &store.TraceDetail{
			TraceID:     "trace-abc",
			ServiceName: "my-svc",
			StartTime:   now,
			EndTime:     now.Add(200 * time.Millisecond),
			DurationMs:  200,
			SpanCount:   3,
			Spans:       []*store.SpanDetail{},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/traces/{trace_id}", handleGetTrace(s))

	w := httptest.NewRecorder()
	r := withTenant(httptest.NewRequest("GET", "/api/v1/traces/trace-abc", nil), "default")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d\nbody: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var body store.TraceDetail
	parseJSONResponse(t, w, &body)
	if body.TraceID != "trace-abc" {
		t.Errorf("trace_id = %q, want %q", body.TraceID, "trace-abc")
	}
}

func TestHandleGetTrace_NotFound(t *testing.T) {
	s := &mockStore{traceDetail: nil}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/traces/{trace_id}", handleGetTrace(s))

	w := httptest.NewRecorder()
	r := withTenant(httptest.NewRequest("GET", "/api/v1/traces/nonexistent", nil), "default")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleListGPUs(t *testing.T) {
	gpuStore := &mockGPUStore{
		gpus: []store.GPUSummary{
			{ResourceUUID: "gpu-1", Model: "A100"},
		},
	}

	handler := handleListGPUs(gpuStore)
	w := httptest.NewRecorder()
	r := withTenant(httptest.NewRequest("GET", "/api/v1/gpus", nil), "default")
	handler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var body map[string]interface{}
	parseJSONResponse(t, w, &body)
	gpus := body["gpus"].([]interface{})
	if len(gpus) != 1 {
		t.Errorf("gpus count = %d, want 1", len(gpus))
	}
}

func TestHandleListGPUs_Empty(t *testing.T) {
	gpuStore := &mockGPUStore{gpus: nil}
	handler := handleListGPUs(gpuStore)

	w := httptest.NewRecorder()
	r := withTenant(httptest.NewRequest("GET", "/api/v1/gpus", nil), "default")
	handler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var body map[string]interface{}
	parseJSONResponse(t, w, &body)
	gpus := body["gpus"].([]interface{})
	if len(gpus) != 0 {
		t.Errorf("gpus count = %d, want 0", len(gpus))
	}
}

func TestHandleGetGPU_Found(t *testing.T) {
	now := time.Now()
	gpuStore := &mockGPUStore{
		detail: &store.GPUDetail{
			ResourceUUID: "gpu-1",
			Model:        "A100",
			FirstSeen:    now,
			LastSeen:     now,
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/gpus/{uuid}", handleGetGPU(gpuStore))

	w := httptest.NewRecorder()
	r := withTenant(httptest.NewRequest("GET", "/api/v1/gpus/gpu-1", nil), "default")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandleGetGPU_NotFound(t *testing.T) {
	gpuStore := &mockGPUStore{detail: nil, getErr: nil}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/gpus/{uuid}", handleGetGPU(gpuStore))

	w := httptest.NewRecorder()
	r := withTenant(httptest.NewRequest("GET", "/api/v1/gpus/nonexistent", nil), "default")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleGetGPU_DBError(t *testing.T) {
	gpuStore := &mockGPUStore{detail: nil, getErr: fmt.Errorf("connection refused")}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/gpus/{uuid}", handleGetGPU(gpuStore))

	w := httptest.NewRecorder()
	r := withTenant(httptest.NewRequest("GET", "/api/v1/gpus/gpu-1", nil), "default")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestHandleAnalyticsOverview(t *testing.T) {
	s := &mockStore{
		overview: &store.AnalyticsOverview{
			TotalTraces:      100,
			AvgLatencyMs:     45.5,
			ErrorRate:        0.02,
			ActiveGPUCount:   4,
			ThroughputSeries: []store.ThroughputPoint{},
			LatencySeries:    []store.LatencyPoint{},
		},
	}

	handler := handleAnalyticsOverview(s)
	w := httptest.NewRecorder()
	r := withTenant(httptest.NewRequest("GET", "/api/v1/analytics/overview", nil), "default")
	handler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var body store.AnalyticsOverview
	parseJSONResponse(t, w, &body)
	if body.TotalTraces != 100 {
		t.Errorf("total_traces = %d, want 100", body.TotalTraces)
	}
}

func TestHandleGetGPUMetrics(t *testing.T) {
	now := time.Now()
	s := &mockStore{
		gpuMetrics: []store.GPUMetricRow{
			{Timestamp: now, ResourceUUID: "gpu-1", Utilization: 85.0, MemoryUsedGB: 12.5, PowerWatts: 300},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/gpus/{uuid}/metrics", handleGetGPUMetrics(s))

	w := httptest.NewRecorder()
	r := withTenant(httptest.NewRequest("GET", "/api/v1/gpus/gpu-1/metrics", nil), "default")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var body map[string]interface{}
	parseJSONResponse(t, w, &body)
	metrics := body["metrics"].([]interface{})
	if len(metrics) != 1 {
		t.Errorf("metrics count = %d, want 1", len(metrics))
	}
}

func TestApiKeyMiddleware_ValidKey(t *testing.T) {
	var capturedTenant string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTenant = tenant.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := apiKeyMiddleware(inner, "test-key-123")
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/traces", nil)
	r.Header.Set("Authorization", "Bearer test-key-123")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if capturedTenant != tenant.DefaultTenantID {
		t.Errorf("tenant = %q, want %q", capturedTenant, tenant.DefaultTenantID)
	}
}

func TestApiKeyMiddleware_InvalidKey(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called with invalid key")
	})

	handler := apiKeyMiddleware(inner, "correct-key")
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/traces", nil)
	r.Header.Set("Authorization", "Bearer wrong-key")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestApiKeyMiddleware_MissingHeader(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called without auth header")
	})

	handler := apiKeyMiddleware(inner, "some-key")
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/traces", nil)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestApiKeyMiddleware_HealthExempt(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := apiKeyMiddleware(inner, "some-key")

	for _, path := range []string{"/healthz", "/readyz"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", path, nil) // no auth header
		handler.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want %d", path, w.Code, http.StatusOK)
		}
	}
}

func TestCorsMiddleware(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := corsMiddleware(inner)

	// Normal request
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/traces", nil)
	handler.ServeHTTP(w, r)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("missing CORS Allow-Origin header")
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestCorsMiddleware_Preflight(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("inner handler should not be called for OPTIONS")
	})

	handler := corsMiddleware(inner)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("OPTIONS", "/api/v1/traces", nil)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("missing CORS Allow-Methods header")
	}
}

func TestAdminKeyMiddleware_ValidKey(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := adminKeyMiddleware(inner, "admin-secret")
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/admin/tenants", nil)
	r.Header.Set("Authorization", "Bearer admin-secret")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAdminKeyMiddleware_InvalidKey(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	handler := adminKeyMiddleware(inner, "admin-secret")
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/admin/tenants", nil)
	r.Header.Set("Authorization", "Bearer wrong-key")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}
