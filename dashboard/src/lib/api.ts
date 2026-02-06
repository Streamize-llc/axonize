import type {
  TracesResponse,
  TraceDetail,
  TraceFilter,
  GPUsResponse,
  GPUDetail,
  GPUMetricsResponse,
  AnalyticsOverview,
} from './types';

const BASE_URL = import.meta.env.VITE_API_URL ?? '';

async function fetchJSON<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`);
  if (!res.ok) {
    throw new Error(`API ${res.status}: ${res.statusText}`);
  }
  return res.json();
}

// ---- Traces ----

export function fetchTraces(filter?: TraceFilter): Promise<TracesResponse> {
  const params = new URLSearchParams();
  if (filter?.service_name) params.set('service_name', filter.service_name);
  if (filter?.start) params.set('start', filter.start);
  if (filter?.end) params.set('end', filter.end);
  if (filter?.limit) params.set('limit', String(filter.limit));
  if (filter?.offset) params.set('offset', String(filter.offset));
  const qs = params.toString();
  return fetchJSON<TracesResponse>(`/api/v1/traces${qs ? `?${qs}` : ''}`);
}

export function fetchTrace(traceId: string): Promise<TraceDetail> {
  return fetchJSON<TraceDetail>(`/api/v1/traces/${traceId}`);
}

// ---- GPUs ----

export function fetchGPUs(): Promise<GPUsResponse> {
  return fetchJSON<GPUsResponse>('/api/v1/gpus');
}

export function fetchGPU(uuid: string): Promise<GPUDetail> {
  return fetchJSON<GPUDetail>(`/api/v1/gpus/${uuid}`);
}

export function fetchGPUMetrics(
  uuid: string,
  start?: string,
  end?: string,
): Promise<GPUMetricsResponse> {
  const params = new URLSearchParams();
  if (start) params.set('start', start);
  if (end) params.set('end', end);
  const qs = params.toString();
  return fetchJSON<GPUMetricsResponse>(
    `/api/v1/gpus/${uuid}/metrics${qs ? `?${qs}` : ''}`,
  );
}

// ---- Analytics ----

export function fetchAnalytics(
  start?: string,
  end?: string,
): Promise<AnalyticsOverview> {
  const params = new URLSearchParams();
  if (start) params.set('start', start);
  if (end) params.set('end', end);
  const qs = params.toString();
  return fetchJSON<AnalyticsOverview>(
    `/api/v1/analytics/overview${qs ? `?${qs}` : ''}`,
  );
}
