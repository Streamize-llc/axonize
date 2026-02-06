package ingest

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"

	collectorpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/axonize/server/internal/store"
)

const (
	defaultBatchSize    = 1000
	defaultFlushTimeout = 1 * time.Second
)

// SpanWriter is the interface for persisting span records.
type SpanWriter interface {
	InsertSpans(ctx context.Context, spans []store.SpanRecord) error
	InsertGPUMetrics(ctx context.Context, spans []store.SpanRecord) error
}

// GPURegistrar is the interface for the GPU registry (PostgreSQL).
type GPURegistrar interface {
	UpsertPhysicalGPU(ctx context.Context, gpu store.PhysicalGPURecord) error
	UpsertComputeResource(ctx context.Context, res store.ComputeResourceRecord) error
	UpsertResourceContext(ctx context.Context, rc store.ResourceContextRecord) error
}

// Handler implements the OTLP TraceService gRPC handler with a batching buffer.
type Handler struct {
	collectorpb.UnimplementedTraceServiceServer

	writer    SpanWriter
	registrar GPURegistrar
	logger    *slog.Logger

	mu        sync.Mutex
	buffer    []store.SpanRecord
	batchSize int

	stopCh chan struct{}
	doneCh chan struct{}
}

// NewHandler creates a new ingest handler.
func NewHandler(writer SpanWriter, registrar GPURegistrar, logger *slog.Logger) *Handler {
	return &Handler{
		writer:    writer,
		registrar: registrar,
		logger:    logger,
		buffer:    make([]store.SpanRecord, 0, defaultBatchSize),
		batchSize: defaultBatchSize,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
}

// Start begins the background flush goroutine.
func (h *Handler) Start() {
	go h.flushLoop()
}

// Stop signals the flush loop to exit and performs a final flush.
func (h *Handler) Stop() {
	close(h.stopCh)
	<-h.doneCh
	h.flush()
}

// Export implements the OTLP TraceService Export RPC.
func (h *Handler) Export(
	ctx context.Context,
	req *collectorpb.ExportTraceServiceRequest,
) (*collectorpb.ExportTraceServiceResponse, error) {
	records := convertRequest(req)

	h.mu.Lock()
	h.buffer = append(h.buffer, records...)
	shouldFlush := len(h.buffer) >= h.batchSize
	h.mu.Unlock()

	if shouldFlush {
		h.flush()
	}

	return &collectorpb.ExportTraceServiceResponse{}, nil
}

func (h *Handler) flushLoop() {
	defer close(h.doneCh)
	ticker := time.NewTicker(defaultFlushTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-h.stopCh:
			return
		case <-ticker.C:
			h.flush()
		}
	}
}

func (h *Handler) flush() {
	h.mu.Lock()
	if len(h.buffer) == 0 {
		h.mu.Unlock()
		return
	}
	batch := h.buffer
	h.buffer = make([]store.SpanRecord, 0, h.batchSize)
	h.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := h.writer.InsertSpans(ctx, batch); err != nil {
		h.logger.Error("failed to flush spans", "count", len(batch), "error", err)
	} else {
		h.logger.Debug("flushed spans", "count", len(batch))
	}

	// Insert GPU metrics into ClickHouse
	if err := h.writer.InsertGPUMetrics(ctx, batch); err != nil {
		h.logger.Error("failed to insert gpu metrics", "error", err)
	}

	// Register GPUs in PostgreSQL
	h.registerGPUs(ctx, batch)
}

// convertRequest transforms an OTLP ExportTraceServiceRequest into SpanRecords.
func convertRequest(req *collectorpb.ExportTraceServiceRequest) []store.SpanRecord {
	var records []store.SpanRecord

	for _, rs := range req.GetResourceSpans() {
		serviceName := ""
		environment := ""
		for _, attr := range rs.GetResource().GetAttributes() {
			switch attr.GetKey() {
			case "service.name":
				serviceName = attr.GetValue().GetStringValue()
			case "deployment.environment":
				environment = attr.GetValue().GetStringValue()
			}
		}

		for _, ss := range rs.GetScopeSpans() {
			for _, span := range ss.GetSpans() {
				record := spanToRecord(span, serviceName, environment)
				records = append(records, record)
			}
		}
	}

	return records
}

func spanToRecord(span *tracepb.Span, serviceName, environment string) store.SpanRecord {
	traceID := hex.EncodeToString(span.GetTraceId())
	spanID := hex.EncodeToString(span.GetSpanId())

	var parentSpanID *string
	if len(span.GetParentSpanId()) > 0 {
		p := hex.EncodeToString(span.GetParentSpanId())
		parentSpanID = &p
	}

	startNano := span.GetStartTimeUnixNano()
	endNano := span.GetEndTimeUnixNano()
	startTime := time.Unix(0, int64(startNano))
	endTime := time.Unix(0, int64(endNano))
	durationMs := float64(endNano-startNano) / 1e6

	status := "ok"
	var errorMessage *string
	switch span.GetStatus().GetCode() {
	case tracepb.Status_STATUS_CODE_ERROR:
		status = "error"
		if msg := span.GetStatus().GetMessage(); msg != "" {
			errorMessage = &msg
		}
	case tracepb.Status_STATUS_CODE_UNSET:
		status = "unset"
	}

	// Extract known attributes; build raw map for GPU parsing
	attrs := make(map[string]string)
	attrMap := make(map[string]*commonpb.AnyValue)
	var modelName, modelVersion, inferenceType *string
	var tokensInput, tokensOutput *uint32
	var tokensPerSecond, ttftMs, cfgScale *float32
	var diffusionSteps *uint16
	var costUSD *float64

	for _, attr := range span.GetAttributes() {
		key := attr.GetKey()
		val := attr.GetValue()
		attrMap[key] = val

		switch key {
		case "ai.model.name":
			s := val.GetStringValue()
			modelName = &s
		case "ai.model.version":
			s := val.GetStringValue()
			modelVersion = &s
		case "ai.inference.type":
			s := val.GetStringValue()
			inferenceType = &s
		case "ai.llm.tokens.input":
			v := uint32(val.GetIntValue())
			tokensInput = &v
		case "ai.llm.tokens.output":
			v := uint32(val.GetIntValue())
			tokensOutput = &v
		case "ai.llm.tokens_per_second":
			v := float32(val.GetDoubleValue())
			tokensPerSecond = &v
		case "ai.llm.ttft_ms":
			v := float32(val.GetDoubleValue())
			ttftMs = &v
		case "ai.diffusion.steps":
			v := uint16(val.GetIntValue())
			diffusionSteps = &v
		case "ai.diffusion.cfg_scale":
			v := float32(val.GetDoubleValue())
			cfgScale = &v
		case "cost.usd":
			v := val.GetDoubleValue()
			costUSD = &v
		default:
			// Skip gpu.N.* from generic attrs — handled by parseGPUAttributes
			if len(key) < 4 || key[:4] != "gpu." {
				attrs[key] = stringifyValue(val)
			}
		}
	}

	record := store.SpanRecord{
		TraceID:         traceID,
		SpanID:          spanID,
		ParentSpanID:    parentSpanID,
		Name:            span.GetName(),
		ServiceName:     serviceName,
		Environment:     environment,
		StartTime:       startTime,
		EndTime:         endTime,
		DurationMs:      durationMs,
		ModelName:       modelName,
		ModelVersion:    modelVersion,
		InferenceType:   inferenceType,
		TokensInput:     tokensInput,
		TokensOutput:    tokensOutput,
		TokensPerSecond: tokensPerSecond,
		TtftMs:          ttftMs,
		DiffusionSteps:  diffusionSteps,
		CfgScale:        cfgScale,
		CostUSD:         costUSD,
		Status:          status,
		ErrorMessage:    errorMessage,
		Attributes:      attrs,
	}

	// Parse GPU indexed attributes (gpu.0.*, gpu.1.*, ...)
	parseGPUAttributes(attrMap, &record)

	return record
}

func parseGPUAttributes(attrs map[string]*commonpb.AnyValue, record *store.SpanRecord) {
	for idx := 0; ; idx++ {
		p := fmt.Sprintf("gpu.%d", idx)
		ruuid, ok := attrs[p+".resource_uuid"]
		if !ok {
			break
		}
		record.GpuResourceUUIDs = append(record.GpuResourceUUIDs, ruuid.GetStringValue())
		if v, ok := attrs[p+".physical_uuid"]; ok {
			record.GpuPhysicalUUIDs = append(record.GpuPhysicalUUIDs, v.GetStringValue())
		}
		if v, ok := attrs[p+".model"]; ok {
			record.GpuModels = append(record.GpuModels, v.GetStringValue())
		}
		if v, ok := attrs[p+".node_id"]; ok {
			record.GpuNodeIDs = append(record.GpuNodeIDs, v.GetStringValue())
		}
		if v, ok := attrs[p+".resource_type"]; ok {
			record.GpuResourceTypes = append(record.GpuResourceTypes, v.GetStringValue())
		}
		if v, ok := attrs[p+".user_label"]; ok {
			record.GpuUserLabels = append(record.GpuUserLabels, v.GetStringValue())
		}
		if v, ok := attrs[p+".memory_used_gb"]; ok {
			record.GpuMemoryUsedGB = append(record.GpuMemoryUsedGB, float32(v.GetDoubleValue()))
		}
		if v, ok := attrs[p+".memory_total_gb"]; ok {
			record.GpuMemoryTotalGB = append(record.GpuMemoryTotalGB, float32(v.GetDoubleValue()))
		}
		if v, ok := attrs[p+".utilization"]; ok {
			record.GpuUtilization = append(record.GpuUtilization, float32(v.GetDoubleValue()))
		}
		if v, ok := attrs[p+".temperature_celsius"]; ok {
			record.GpuTemperatureCels = append(record.GpuTemperatureCels, uint8(v.GetIntValue()))
		}
		if v, ok := attrs[p+".power_watts"]; ok {
			record.GpuPowerWatts = append(record.GpuPowerWatts, uint16(v.GetIntValue()))
		}
		if v, ok := attrs[p+".clock_mhz"]; ok {
			record.GpuClockMHz = append(record.GpuClockMHz, uint16(v.GetIntValue()))
		}
	}
}

func stringifyValue(val *commonpb.AnyValue) string {
	switch v := val.GetValue().(type) {
	case *commonpb.AnyValue_StringValue:
		return v.StringValue
	case *commonpb.AnyValue_IntValue:
		return fmt.Sprintf("%d", v.IntValue)
	case *commonpb.AnyValue_DoubleValue:
		return fmt.Sprintf("%g", v.DoubleValue)
	case *commonpb.AnyValue_BoolValue:
		return fmt.Sprintf("%t", v.BoolValue)
	default:
		return ""
	}
}

// registerGPUs upserts GPU identity info into the PostgreSQL registry.
func (h *Handler) registerGPUs(ctx context.Context, batch []store.SpanRecord) {
	if h.registrar == nil {
		return
	}

	// Deduplicate within this batch
	seen := make(map[string]bool)

	for _, span := range batch {
		for i, resourceUUID := range span.GpuResourceUUIDs {
			if seen[resourceUUID] {
				continue
			}
			seen[resourceUUID] = true

			var physicalUUID, model, nodeID, resourceType string
			var memTotalGB float32
			if i < len(span.GpuPhysicalUUIDs) {
				physicalUUID = span.GpuPhysicalUUIDs[i]
			}
			if i < len(span.GpuModels) {
				model = span.GpuModels[i]
			}
			if i < len(span.GpuNodeIDs) {
				nodeID = span.GpuNodeIDs[i]
			}
			if i < len(span.GpuResourceTypes) {
				resourceType = span.GpuResourceTypes[i]
			} else if resourceUUID != physicalUUID {
				resourceType = "mig"
			} else {
				resourceType = "full_gpu"
			}
			if i < len(span.GpuMemoryTotalGB) {
				memTotalGB = span.GpuMemoryTotalGB[i]
			}

			// Derive vendor from model name
			vendor := "NVIDIA"

			if err := h.registrar.UpsertPhysicalGPU(ctx, store.PhysicalGPURecord{
				UUID:          physicalUUID,
				Model:         model,
				Vendor:        vendor,
				MemoryTotalGB: memTotalGB,
				NodeID:        nodeID,
			}); err != nil {
				h.logger.Error("failed to upsert physical gpu", "uuid", physicalUUID, "error", err)
			}

			if err := h.registrar.UpsertComputeResource(ctx, store.ComputeResourceRecord{
				ResourceUUID: resourceUUID,
				PhysicalUUID: physicalUUID,
				ResourceType: resourceType,
				MemoryGB:     memTotalGB,
			}); err != nil {
				h.logger.Error("failed to upsert compute resource", "uuid", resourceUUID, "error", err)
			}

			// Upsert resource context (runtime label mapping)
			var userLabel string
			if i < len(span.GpuUserLabels) {
				userLabel = span.GpuUserLabels[i]
			}
			if userLabel != "" {
				if err := h.registrar.UpsertResourceContext(ctx, store.ResourceContextRecord{
					ResourceUUID: resourceUUID,
					UserLabel:    userLabel,
					Hostname:     nodeID,
				}); err != nil {
					h.logger.Error("failed to upsert resource context", "uuid", resourceUUID, "error", err)
				}
			}
		}
	}
}

