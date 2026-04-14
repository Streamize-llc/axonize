package ingest

import (
	"context"
	"encoding/hex"
	"log/slog"
	"sync"
	"testing"
	"time"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	collectorpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/axonize/server/internal/store"
	"github.com/axonize/server/internal/tenant"
)

// --- Mocks ---

type mockSpanWriter struct {
	mu    sync.Mutex
	spans []store.SpanRecord
	err   error
}

func (m *mockSpanWriter) InsertSpans(_ context.Context, spans []store.SpanRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.spans = append(m.spans, spans...)
	return m.err
}

func (m *mockSpanWriter) InsertGPUMetrics(_ context.Context, _ []store.SpanRecord) error {
	return m.err
}

type mockGPURegistrar struct {
	mu          sync.Mutex
	physicals   []store.PhysicalGPURecord
	computes    []store.ComputeResourceRecord
	contexts    []store.ResourceContextRecord
}

func (m *mockGPURegistrar) UpsertPhysicalGPU(_ context.Context, gpu store.PhysicalGPURecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.physicals = append(m.physicals, gpu)
	return nil
}

func (m *mockGPURegistrar) UpsertComputeResource(_ context.Context, res store.ComputeResourceRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.computes = append(m.computes, res)
	return nil
}

func (m *mockGPURegistrar) UpsertResourceContext(_ context.Context, rc store.ResourceContextRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.contexts = append(m.contexts, rc)
	return nil
}

type mockUsageRecorder struct {
	mu         sync.Mutex
	spanCounts map[string]int
	gpuSeconds map[string]int64
}

func newMockUsageRecorder() *mockUsageRecorder {
	return &mockUsageRecorder{
		spanCounts: make(map[string]int),
		gpuSeconds: make(map[string]int64),
	}
}

func (m *mockUsageRecorder) RecordSpans(_ context.Context, tenantID string, count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.spanCounts[tenantID] += count
}

func (m *mockUsageRecorder) RecordGPUSeconds(_ context.Context, tenantID string, seconds int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gpuSeconds[tenantID] += seconds
}

// --- Helpers ---

func makeTraceID() []byte {
	b, _ := hex.DecodeString("0102030405060708090a0b0c0d0e0f10")
	return b
}

func makeSpanID() []byte {
	b, _ := hex.DecodeString("0102030405060708")
	return b
}

func makeOTLPRequest(serviceName string, spans []*tracepb.Span) *collectorpb.ExportTraceServiceRequest {
	return &collectorpb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: serviceName}}},
						{Key: "deployment.environment", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "production"}}},
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{Spans: spans},
				},
			},
		},
	}
}

func makeSpan(name string, attrs []*commonpb.KeyValue) *tracepb.Span {
	now := time.Now()
	return &tracepb.Span{
		TraceId:            makeTraceID(),
		SpanId:             makeSpanID(),
		Name:               name,
		StartTimeUnixNano:  uint64(now.UnixNano()),
		EndTimeUnixNano:    uint64(now.Add(100 * time.Millisecond).UnixNano()),
		Status:             &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
		Attributes:         attrs,
	}
}

func kvStr(key, val string) *commonpb.KeyValue {
	return &commonpb.KeyValue{
		Key:   key,
		Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: val}},
	}
}

func kvInt(key string, val int64) *commonpb.KeyValue {
	return &commonpb.KeyValue{
		Key:   key,
		Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: val}},
	}
}

func kvDouble(key string, val float64) *commonpb.KeyValue {
	return &commonpb.KeyValue{
		Key:   key,
		Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_DoubleValue{DoubleValue: val}},
	}
}

// --- Tests ---

func TestConvertRequest_BasicSpan(t *testing.T) {
	span := makeSpan("inference", []*commonpb.KeyValue{
		kvStr("ai.model.name", "llama-3"),
		kvStr("ai.inference.type", "llm"),
		kvInt("ai.llm.tokens.input", 128),
		kvInt("ai.llm.tokens.output", 256),
	})

	ctx := tenant.WithTenantID(context.Background(), "tn_test")
	req := makeOTLPRequest("my-service", []*tracepb.Span{span})
	records := convertRequest(ctx, req)

	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}

	r := records[0]
	if r.TenantID != "tn_test" {
		t.Errorf("TenantID = %q, want %q", r.TenantID, "tn_test")
	}
	if r.ServiceName != "my-service" {
		t.Errorf("ServiceName = %q, want %q", r.ServiceName, "my-service")
	}
	if r.Environment != "production" {
		t.Errorf("Environment = %q, want %q", r.Environment, "production")
	}
	if r.Name != "inference" {
		t.Errorf("Name = %q, want %q", r.Name, "inference")
	}
	if r.ModelName == nil || *r.ModelName != "llama-3" {
		t.Errorf("ModelName = %v, want %q", r.ModelName, "llama-3")
	}
	if r.TokensInput == nil || *r.TokensInput != 128 {
		t.Errorf("TokensInput = %v, want 128", r.TokensInput)
	}
	if r.TokensOutput == nil || *r.TokensOutput != 256 {
		t.Errorf("TokensOutput = %v, want 256", r.TokensOutput)
	}
	if r.Status != "ok" {
		t.Errorf("Status = %q, want %q", r.Status, "ok")
	}
	if r.TraceID != hex.EncodeToString(makeTraceID()) {
		t.Errorf("TraceID = %q, want hex-encoded trace ID", r.TraceID)
	}
}

func TestConvertRequest_DefaultTenantID(t *testing.T) {
	span := makeSpan("test", nil)
	ctx := context.Background() // no tenant set
	req := makeOTLPRequest("svc", []*tracepb.Span{span})
	records := convertRequest(ctx, req)

	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0].TenantID != tenant.DefaultTenantID {
		t.Errorf("TenantID = %q, want %q", records[0].TenantID, tenant.DefaultTenantID)
	}
}

func TestConvertRequest_ErrorStatus(t *testing.T) {
	now := time.Now()
	span := &tracepb.Span{
		TraceId:           makeTraceID(),
		SpanId:            makeSpanID(),
		Name:              "failed-inference",
		StartTimeUnixNano: uint64(now.UnixNano()),
		EndTimeUnixNano:   uint64(now.Add(50 * time.Millisecond).UnixNano()),
		Status: &tracepb.Status{
			Code:    tracepb.Status_STATUS_CODE_ERROR,
			Message: "out of memory",
		},
	}

	ctx := context.Background()
	req := makeOTLPRequest("svc", []*tracepb.Span{span})
	records := convertRequest(ctx, req)

	r := records[0]
	if r.Status != "error" {
		t.Errorf("Status = %q, want %q", r.Status, "error")
	}
	if r.ErrorMessage == nil || *r.ErrorMessage != "out of memory" {
		t.Errorf("ErrorMessage = %v, want %q", r.ErrorMessage, "out of memory")
	}
}

func TestConvertRequest_UnsetStatus(t *testing.T) {
	now := time.Now()
	span := &tracepb.Span{
		TraceId:           makeTraceID(),
		SpanId:            makeSpanID(),
		Name:              "unset",
		StartTimeUnixNano: uint64(now.UnixNano()),
		EndTimeUnixNano:   uint64(now.Add(10 * time.Millisecond).UnixNano()),
		Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_UNSET},
	}

	ctx := context.Background()
	req := makeOTLPRequest("svc", []*tracepb.Span{span})
	records := convertRequest(ctx, req)

	if records[0].Status != "unset" {
		t.Errorf("Status = %q, want %q", records[0].Status, "unset")
	}
}

func TestParseGPUAttributes(t *testing.T) {
	attrs := map[string]*commonpb.AnyValue{
		"gpu.0.resource_uuid": {Value: &commonpb.AnyValue_StringValue{StringValue: "gpu-uuid-0"}},
		"gpu.0.physical_uuid": {Value: &commonpb.AnyValue_StringValue{StringValue: "phys-uuid-0"}},
		"gpu.0.model":         {Value: &commonpb.AnyValue_StringValue{StringValue: "A100"}},
		"gpu.0.vendor":        {Value: &commonpb.AnyValue_StringValue{StringValue: "NVIDIA"}},
		"gpu.0.node_id":       {Value: &commonpb.AnyValue_StringValue{StringValue: "node-1"}},
		"gpu.0.memory_used_gb":  {Value: &commonpb.AnyValue_DoubleValue{DoubleValue: 12.5}},
		"gpu.0.memory_total_gb": {Value: &commonpb.AnyValue_DoubleValue{DoubleValue: 80.0}},
		"gpu.0.utilization":     {Value: &commonpb.AnyValue_DoubleValue{DoubleValue: 85.0}},
		"gpu.0.temperature_celsius": {Value: &commonpb.AnyValue_IntValue{IntValue: 72}},
		"gpu.0.power_watts":    {Value: &commonpb.AnyValue_IntValue{IntValue: 300}},
		"gpu.0.clock_mhz":     {Value: &commonpb.AnyValue_IntValue{IntValue: 1410}},
		"gpu.0.resource_type":  {Value: &commonpb.AnyValue_StringValue{StringValue: "full_gpu"}},
		"gpu.0.user_label":     {Value: &commonpb.AnyValue_StringValue{StringValue: "cuda:0"}},
		"gpu.1.resource_uuid":  {Value: &commonpb.AnyValue_StringValue{StringValue: "gpu-uuid-1"}},
		"gpu.1.physical_uuid":  {Value: &commonpb.AnyValue_StringValue{StringValue: "phys-uuid-1"}},
		"gpu.1.model":          {Value: &commonpb.AnyValue_StringValue{StringValue: "A100"}},
		"gpu.1.vendor":         {Value: &commonpb.AnyValue_StringValue{StringValue: "NVIDIA"}},
	}

	var record store.SpanRecord
	parseGPUAttributes(attrs, &record)

	if len(record.GpuResourceUUIDs) != 2 {
		t.Fatalf("GpuResourceUUIDs = %d, want 2", len(record.GpuResourceUUIDs))
	}
	if record.GpuResourceUUIDs[0] != "gpu-uuid-0" {
		t.Errorf("GpuResourceUUIDs[0] = %q, want %q", record.GpuResourceUUIDs[0], "gpu-uuid-0")
	}
	if record.GpuResourceUUIDs[1] != "gpu-uuid-1" {
		t.Errorf("GpuResourceUUIDs[1] = %q, want %q", record.GpuResourceUUIDs[1], "gpu-uuid-1")
	}
	if record.GpuModels[0] != "A100" {
		t.Errorf("GpuModels[0] = %q, want %q", record.GpuModels[0], "A100")
	}
	if record.GpuVendors[0] != "NVIDIA" {
		t.Errorf("GpuVendors[0] = %q, want %q", record.GpuVendors[0], "NVIDIA")
	}
	if record.GpuMemoryUsedGB[0] != 12.5 {
		t.Errorf("GpuMemoryUsedGB[0] = %f, want 12.5", record.GpuMemoryUsedGB[0])
	}
	if record.GpuUtilization[0] != 85.0 {
		t.Errorf("GpuUtilization[0] = %f, want 85.0", record.GpuUtilization[0])
	}
	if record.GpuTemperatureCels[0] != 72 {
		t.Errorf("GpuTemperatureCels[0] = %d, want 72", record.GpuTemperatureCels[0])
	}
	if record.GpuPowerWatts[0] != 300 {
		t.Errorf("GpuPowerWatts[0] = %d, want 300", record.GpuPowerWatts[0])
	}
	if record.GpuClockMHz[0] != 1410 {
		t.Errorf("GpuClockMHz[0] = %d, want 1410", record.GpuClockMHz[0])
	}
	if record.GpuResourceTypes[0] != "full_gpu" {
		t.Errorf("GpuResourceTypes[0] = %q, want %q", record.GpuResourceTypes[0], "full_gpu")
	}
	if record.GpuUserLabels[0] != "cuda:0" {
		t.Errorf("GpuUserLabels[0] = %q, want %q", record.GpuUserLabels[0], "cuda:0")
	}
}

func TestParseGPUAttributes_NoGPUs(t *testing.T) {
	attrs := map[string]*commonpb.AnyValue{
		"custom.key": {Value: &commonpb.AnyValue_StringValue{StringValue: "value"}},
	}

	var record store.SpanRecord
	parseGPUAttributes(attrs, &record)

	if len(record.GpuResourceUUIDs) != 0 {
		t.Errorf("GpuResourceUUIDs = %d, want 0", len(record.GpuResourceUUIDs))
	}
}

func TestConvertRequest_DiffusionAttributes(t *testing.T) {
	span := makeSpan("generate-image", []*commonpb.KeyValue{
		kvStr("ai.model.name", "sdxl"),
		kvStr("ai.inference.type", "diffusion"),
		kvInt("ai.diffusion.steps", 30),
		kvDouble("ai.diffusion.cfg_scale", 7.5),
	})

	ctx := context.Background()
	req := makeOTLPRequest("diffusion-svc", []*tracepb.Span{span})
	records := convertRequest(ctx, req)

	r := records[0]
	if r.DiffusionSteps == nil || *r.DiffusionSteps != 30 {
		t.Errorf("DiffusionSteps = %v, want 30", r.DiffusionSteps)
	}
	if r.CfgScale == nil || *r.CfgScale != 7.5 {
		t.Errorf("CfgScale = %v, want 7.5", r.CfgScale)
	}
}

func TestConvertRequest_CostUSD(t *testing.T) {
	span := makeSpan("inference", []*commonpb.KeyValue{
		kvDouble("cost.usd", 0.0035),
	})

	ctx := context.Background()
	req := makeOTLPRequest("svc", []*tracepb.Span{span})
	records := convertRequest(ctx, req)

	r := records[0]
	if r.CostUSD == nil || *r.CostUSD != 0.0035 {
		t.Errorf("CostUSD = %v, want 0.0035", r.CostUSD)
	}
}

func TestConvertRequest_ParentSpanID(t *testing.T) {
	parentID, _ := hex.DecodeString("aabbccdd11223344")
	now := time.Now()
	span := &tracepb.Span{
		TraceId:           makeTraceID(),
		SpanId:            makeSpanID(),
		ParentSpanId:      parentID,
		Name:              "child",
		StartTimeUnixNano: uint64(now.UnixNano()),
		EndTimeUnixNano:   uint64(now.Add(10 * time.Millisecond).UnixNano()),
		Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
	}

	ctx := context.Background()
	req := makeOTLPRequest("svc", []*tracepb.Span{span})
	records := convertRequest(ctx, req)

	r := records[0]
	if r.ParentSpanID == nil {
		t.Fatal("ParentSpanID is nil, want non-nil")
	}
	want := hex.EncodeToString(parentID)
	if *r.ParentSpanID != want {
		t.Errorf("ParentSpanID = %q, want %q", *r.ParentSpanID, want)
	}
}

func TestConvertRequest_NoParentSpanID(t *testing.T) {
	span := makeSpan("root", nil)

	ctx := context.Background()
	req := makeOTLPRequest("svc", []*tracepb.Span{span})
	records := convertRequest(ctx, req)

	if records[0].ParentSpanID != nil {
		t.Errorf("ParentSpanID = %v, want nil", records[0].ParentSpanID)
	}
}

func TestConvertRequest_DurationMs(t *testing.T) {
	now := time.Now()
	span := &tracepb.Span{
		TraceId:           makeTraceID(),
		SpanId:            makeSpanID(),
		Name:              "timed",
		StartTimeUnixNano: uint64(now.UnixNano()),
		EndTimeUnixNano:   uint64(now.Add(250 * time.Millisecond).UnixNano()),
		Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
	}

	ctx := context.Background()
	req := makeOTLPRequest("svc", []*tracepb.Span{span})
	records := convertRequest(ctx, req)

	r := records[0]
	if r.DurationMs < 249 || r.DurationMs > 251 {
		t.Errorf("DurationMs = %f, want ~250", r.DurationMs)
	}
}

func TestConvertRequest_CustomAttributes(t *testing.T) {
	span := makeSpan("inference", []*commonpb.KeyValue{
		kvStr("ai.model.name", "gpt-4"),
		kvStr("custom.tag", "experiment-1"),
		kvInt("custom.batch_size", 32),
	})

	ctx := context.Background()
	req := makeOTLPRequest("svc", []*tracepb.Span{span})
	records := convertRequest(ctx, req)

	r := records[0]
	if v, ok := r.Attributes["custom.tag"]; !ok || v != "experiment-1" {
		t.Errorf("Attributes[custom.tag] = %q, want %q", v, "experiment-1")
	}
	if v, ok := r.Attributes["custom.batch_size"]; !ok || v != "32" {
		t.Errorf("Attributes[custom.batch_size] = %q, want %q", v, "32")
	}
	// Known AI attributes should NOT be in generic attributes map
	if _, ok := r.Attributes["ai.model.name"]; ok {
		t.Error("ai.model.name should not be in generic Attributes map")
	}
}

func TestConvertRequest_GPUAttributesNotInGenericMap(t *testing.T) {
	span := makeSpan("inference", []*commonpb.KeyValue{
		kvStr("gpu.0.resource_uuid", "uuid-0"),
		kvStr("gpu.0.model", "A100"),
		kvStr("custom.key", "value"),
	})

	ctx := context.Background()
	req := makeOTLPRequest("svc", []*tracepb.Span{span})
	records := convertRequest(ctx, req)

	r := records[0]
	if _, ok := r.Attributes["gpu.0.resource_uuid"]; ok {
		t.Error("gpu.0.resource_uuid should not be in generic Attributes map")
	}
	if _, ok := r.Attributes["custom.key"]; !ok {
		t.Error("custom.key should be in generic Attributes map")
	}
}

func TestStringifyValue(t *testing.T) {
	tests := []struct {
		name string
		val  *commonpb.AnyValue
		want string
	}{
		{"string", &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "hello"}}, "hello"},
		{"int", &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: 42}}, "42"},
		{"double", &commonpb.AnyValue{Value: &commonpb.AnyValue_DoubleValue{DoubleValue: 3.14}}, "3.14"},
		{"bool", &commonpb.AnyValue{Value: &commonpb.AnyValue_BoolValue{BoolValue: true}}, "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stringifyValue(tt.val)
			if got != tt.want {
				t.Errorf("stringifyValue = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRecordUsage(t *testing.T) {
	meter := newMockUsageRecorder()
	h := &Handler{meter: meter}

	batch := []store.SpanRecord{
		{TenantID: "tn_a", DurationMs: 5000, GpuResourceUUIDs: []string{"gpu-1", "gpu-2"}},
		{TenantID: "tn_a", DurationMs: 3000},
		{TenantID: "tn_b", DurationMs: 2000, GpuResourceUUIDs: []string{"gpu-3"}},
	}

	h.recordUsage(context.Background(), batch)

	if meter.spanCounts["tn_a"] != 2 {
		t.Errorf("tn_a span count = %d, want 2", meter.spanCounts["tn_a"])
	}
	if meter.spanCounts["tn_b"] != 1 {
		t.Errorf("tn_b span count = %d, want 1", meter.spanCounts["tn_b"])
	}
	// tn_a: 5s * 2 GPUs = 10 GPU-seconds
	if meter.gpuSeconds["tn_a"] != 10 {
		t.Errorf("tn_a gpu seconds = %d, want 10", meter.gpuSeconds["tn_a"])
	}
	// tn_b: 2s * 1 GPU = 2 GPU-seconds
	if meter.gpuSeconds["tn_b"] != 2 {
		t.Errorf("tn_b gpu seconds = %d, want 2", meter.gpuSeconds["tn_b"])
	}
}

func TestRecordUsage_MinOneSecond(t *testing.T) {
	meter := newMockUsageRecorder()
	h := &Handler{meter: meter}

	batch := []store.SpanRecord{
		{TenantID: "tn_a", DurationMs: 100, GpuResourceUUIDs: []string{"gpu-1"}}, // < 1 second
	}

	h.recordUsage(context.Background(), batch)

	// Minimum 1 second per span
	if meter.gpuSeconds["tn_a"] != 1 {
		t.Errorf("tn_a gpu seconds = %d, want 1 (minimum)", meter.gpuSeconds["tn_a"])
	}
}

func TestRegisterGPUs(t *testing.T) {
	reg := &mockGPURegistrar{}
	h := &Handler{
		registrar: reg,
		logger:    noopLogger(),
	}

	batch := []store.SpanRecord{
		{
			TenantID:         "tn_test",
			GpuResourceUUIDs: []string{"res-uuid-1"},
			GpuPhysicalUUIDs: []string{"phys-uuid-1"},
			GpuModels:        []string{"A100"},
			GpuVendors:       []string{"NVIDIA"},
			GpuNodeIDs:       []string{"node-1"},
			GpuResourceTypes: []string{"full_gpu"},
			GpuMemoryTotalGB: []float32{80.0},
			GpuUserLabels:    []string{"cuda:0"},
		},
	}

	h.registerGPUs(context.Background(), batch)

	if len(reg.physicals) != 1 {
		t.Fatalf("physicals = %d, want 1", len(reg.physicals))
	}
	p := reg.physicals[0]
	if p.TenantID != "tn_test" || p.UUID != "phys-uuid-1" || p.Model != "A100" || p.Vendor != "NVIDIA" {
		t.Errorf("physical GPU mismatch: %+v", p)
	}

	if len(reg.computes) != 1 {
		t.Fatalf("computes = %d, want 1", len(reg.computes))
	}
	c := reg.computes[0]
	if c.ResourceUUID != "res-uuid-1" || c.PhysicalUUID != "phys-uuid-1" || c.ResourceType != "full_gpu" {
		t.Errorf("compute resource mismatch: %+v", c)
	}

	if len(reg.contexts) != 1 {
		t.Fatalf("contexts = %d, want 1", len(reg.contexts))
	}
	if reg.contexts[0].UserLabel != "cuda:0" {
		t.Errorf("context user label = %q, want %q", reg.contexts[0].UserLabel, "cuda:0")
	}
}

func TestRegisterGPUs_Deduplication(t *testing.T) {
	reg := &mockGPURegistrar{}
	h := &Handler{
		registrar: reg,
		logger:    noopLogger(),
	}

	batch := []store.SpanRecord{
		{TenantID: "tn_test", GpuResourceUUIDs: []string{"res-1"}, GpuPhysicalUUIDs: []string{"phys-1"}, GpuModels: []string{"A100"}, GpuVendors: []string{"NVIDIA"}},
		{TenantID: "tn_test", GpuResourceUUIDs: []string{"res-1"}, GpuPhysicalUUIDs: []string{"phys-1"}, GpuModels: []string{"A100"}, GpuVendors: []string{"NVIDIA"}},
	}

	h.registerGPUs(context.Background(), batch)

	if len(reg.physicals) != 1 {
		t.Errorf("physicals = %d, want 1 (deduped)", len(reg.physicals))
	}
	if len(reg.computes) != 1 {
		t.Errorf("computes = %d, want 1 (deduped)", len(reg.computes))
	}
}

func TestRegisterGPUs_DefaultVendor(t *testing.T) {
	reg := &mockGPURegistrar{}
	h := &Handler{
		registrar: reg,
		logger:    noopLogger(),
	}

	batch := []store.SpanRecord{
		{TenantID: "tn_test", GpuResourceUUIDs: []string{"res-1"}, GpuPhysicalUUIDs: []string{"phys-1"}, GpuModels: []string{"V100"}},
	}

	h.registerGPUs(context.Background(), batch)

	if reg.physicals[0].Vendor != "NVIDIA" {
		t.Errorf("default vendor = %q, want %q", reg.physicals[0].Vendor, "NVIDIA")
	}
}

func TestRegisterGPUs_MIGAutoDetect(t *testing.T) {
	reg := &mockGPURegistrar{}
	h := &Handler{
		registrar: reg,
		logger:    noopLogger(),
	}

	batch := []store.SpanRecord{
		{
			TenantID:         "tn_test",
			GpuResourceUUIDs: []string{"mig-uuid-1"},
			GpuPhysicalUUIDs: []string{"phys-uuid-1"}, // different from resource UUID → MIG
			GpuModels:        []string{"A100"},
		},
	}

	h.registerGPUs(context.Background(), batch)

	if reg.computes[0].ResourceType != "mig" {
		t.Errorf("ResourceType = %q, want %q", reg.computes[0].ResourceType, "mig")
	}
}

func TestRegisterGPUs_FullGPUAutoDetect(t *testing.T) {
	reg := &mockGPURegistrar{}
	h := &Handler{
		registrar: reg,
		logger:    noopLogger(),
	}

	batch := []store.SpanRecord{
		{
			TenantID:         "tn_test",
			GpuResourceUUIDs: []string{"same-uuid"},
			GpuPhysicalUUIDs: []string{"same-uuid"}, // same as resource UUID → full GPU
			GpuModels:        []string{"RTX4090"},
		},
	}

	h.registerGPUs(context.Background(), batch)

	if reg.computes[0].ResourceType != "full_gpu" {
		t.Errorf("ResourceType = %q, want %q", reg.computes[0].ResourceType, "full_gpu")
	}
}

func TestRegisterGPUs_NilRegistrar(t *testing.T) {
	h := &Handler{registrar: nil}
	// Should not panic
	h.registerGPUs(context.Background(), []store.SpanRecord{
		{GpuResourceUUIDs: []string{"uuid"}},
	})
}

func noopLogger() *slog.Logger {
	return slog.Default()
}
