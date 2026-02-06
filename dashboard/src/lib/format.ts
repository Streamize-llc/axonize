import { format, formatDistanceToNow } from 'date-fns';

export function formatDuration(ms: number): string {
  if (ms < 1) return `${(ms * 1000).toFixed(0)}us`;
  if (ms < 1000) return `${ms.toFixed(1)}ms`;
  return `${(ms / 1000).toFixed(2)}s`;
}

export function formatTimestamp(iso: string): string {
  return format(new Date(iso), 'yyyy-MM-dd HH:mm:ss');
}

export function formatRelative(iso: string): string {
  return formatDistanceToNow(new Date(iso), { addSuffix: true });
}

export function formatPercent(value: number): string {
  return `${value.toFixed(1)}%`;
}

export function formatGB(value: number): string {
  return `${value.toFixed(1)} GB`;
}
