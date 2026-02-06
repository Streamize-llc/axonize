// ---- Trace types ----

export interface TraceSummary {
  trace_id: string;
  start_time: string;
  end_time: string;
  duration_ms: number;
  service_name: string;
  environment: string;
  span_count: number;
  error_count: number;
}

export interface SpanDetail {
  span_id: string;
  parent_span_id?: string;
  name: string;
  start_time: string;
  end_time: string;
  duration_ms: number;
  status: string;
  error_message?: string;
  attributes?: Record<string, string>;
  children: SpanDetail[];
}

export interface TraceDetail {
  trace_id: string;
  start_time: string;
  end_time: string;
  duration_ms: number;
  service_name: string;
  environment: string;
  span_count: number;
  error_count: number;
  spans: SpanDetail[];
}

export interface TraceFilter {
  service_name?: string;
  start?: string;
  end?: string;
  limit?: number;
  offset?: number;
}

export interface TracesResponse {
  traces: TraceSummary[];
  total: number;
  limit: number;
  offset: number;
}

// ---- GPU types ----

export interface GPUSummary {
  resource_uuid: string;
  physical_uuid: string;
  model: string;
  resource_type: string;
  node_id: string;
  utilization: number;
  memory_used_gb: number;
  memory_total_gb: number;
}

export interface GPUDetail {
  resource_uuid: string;
  physical_uuid: string;
  model: string;
  resource_type: string;
  node_id: string;
  first_seen: string;
  last_seen: string;
}

export interface GPUMetricRow {
  timestamp: string;
  resource_uuid: string;
  utilization: number;
  memory_used_gb: number;
  power_watts: number;
}

export interface GPUsResponse {
  gpus: GPUSummary[];
}

export interface GPUMetricsResponse {
  metrics: GPUMetricRow[];
}

// ---- Analytics types ----

export interface AnalyticsOverview {
  total_traces: number;
  avg_latency_ms: number;
  error_rate: number;
  active_gpu_count: number;
  throughput_series: ThroughputPoint[];
  latency_series: LatencyPoint[];
}

export interface ThroughputPoint {
  timestamp: string;
  count: number;
}

export interface LatencyPoint {
  timestamp: string;
  p50_ms: number;
  p95_ms: number;
  p99_ms: number;
}
