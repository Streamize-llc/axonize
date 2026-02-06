import { useQuery } from '@tanstack/react-query';
import type { TraceFilter } from '../lib/types';
import {
  fetchTraces,
  fetchTrace,
  fetchGPUs,
  fetchGPU,
  fetchGPUMetrics,
  fetchAnalytics,
} from '../lib/api';

export function useTraces(filter?: TraceFilter) {
  return useQuery({
    queryKey: ['traces', filter],
    queryFn: () => fetchTraces(filter),
    refetchInterval: 10_000,
  });
}

export function useTrace(traceId: string) {
  return useQuery({
    queryKey: ['trace', traceId],
    queryFn: () => fetchTrace(traceId),
    enabled: !!traceId,
  });
}

export function useGPUs() {
  return useQuery({
    queryKey: ['gpus'],
    queryFn: fetchGPUs,
    refetchInterval: 10_000,
  });
}

export function useGPU(uuid: string) {
  return useQuery({
    queryKey: ['gpu', uuid],
    queryFn: () => fetchGPU(uuid),
    enabled: !!uuid,
  });
}

export function useGPUMetrics(uuid: string, start?: string, end?: string) {
  return useQuery({
    queryKey: ['gpuMetrics', uuid, start, end],
    queryFn: () => fetchGPUMetrics(uuid, start, end),
    enabled: !!uuid,
    refetchInterval: 10_000,
  });
}

export function useAnalytics(start?: string, end?: string) {
  return useQuery({
    queryKey: ['analytics', start, end],
    queryFn: () => fetchAnalytics(start, end),
    refetchInterval: 15_000,
  });
}
