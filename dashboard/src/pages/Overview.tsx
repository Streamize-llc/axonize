import {
  Activity,
  Clock,
  AlertCircle,
  Cpu,
} from 'lucide-react';
import {
  ResponsiveContainer,
  AreaChart,
  Area,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
} from 'recharts';
import { useAnalytics } from '../hooks/useAPI';
import { Loading } from '../components/common/Loading';
import { ErrorState } from '../components/common/ErrorState';
import { cn } from '../lib/cn';

function StatCard({
  label,
  value,
  icon: Icon,
  color,
}: {
  label: string;
  value: string;
  icon: React.ComponentType<{ className?: string }>;
  color: string;
}) {
  return (
    <div className="rounded-xl border border-border bg-surface-light p-5">
      <div className="flex items-center justify-between">
        <p className="text-sm text-text-secondary">{label}</p>
        <div className={cn('rounded-lg p-2', color)}>
          <Icon className="h-4 w-4" />
        </div>
      </div>
      <p className="mt-2 text-2xl font-semibold">{value}</p>
    </div>
  );
}

export function Overview() {
  const { data, isLoading, error, refetch } = useAnalytics();

  if (isLoading) return <Loading message="Loading overview..." />;
  if (error) return <ErrorState message={error.message} onRetry={refetch} />;

  const stats = data ?? {
    total_traces: 0,
    avg_latency_ms: 0,
    error_rate: 0,
    active_gpu_count: 0,
    throughput_series: [],
    latency_series: [],
  };

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Overview</h1>

      {/* Summary cards */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          label="Total Traces"
          value={stats.total_traces.toLocaleString()}
          icon={Activity}
          color="bg-primary/15 text-primary"
        />
        <StatCard
          label="Avg Latency"
          value={`${stats.avg_latency_ms.toFixed(1)}ms`}
          icon={Clock}
          color="bg-warning/15 text-warning"
        />
        <StatCard
          label="Error Rate"
          value={`${(stats.error_rate * 100).toFixed(1)}%`}
          icon={AlertCircle}
          color="bg-error/15 text-error"
        />
        <StatCard
          label="Active GPUs"
          value={String(stats.active_gpu_count)}
          icon={Cpu}
          color="bg-success/15 text-success"
        />
      </div>

      {/* Throughput chart */}
      {stats.throughput_series.length > 0 && (
        <div className="rounded-xl border border-border bg-surface-light p-5">
          <h2 className="mb-4 text-sm font-medium text-text-secondary">
            Inference Throughput
          </h2>
          <ResponsiveContainer width="100%" height={280}>
            <AreaChart data={stats.throughput_series}>
              <CartesianGrid strokeDasharray="3 3" stroke="#374151" />
              <XAxis
                dataKey="timestamp"
                tick={{ fill: '#9ca3af', fontSize: 12 }}
                tickFormatter={(v) => new Date(v).toLocaleTimeString()}
              />
              <YAxis tick={{ fill: '#9ca3af', fontSize: 12 }} />
              <Tooltip
                contentStyle={{
                  backgroundColor: '#1f2937',
                  border: '1px solid #374151',
                  borderRadius: 8,
                  color: '#f9fafb',
                }}
              />
              <Area
                type="monotone"
                dataKey="count"
                stroke="#6366f1"
                fill="#6366f1"
                fillOpacity={0.15}
                name="Traces"
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      )}

      {/* Latency chart */}
      {stats.latency_series.length > 0 && (
        <div className="rounded-xl border border-border bg-surface-light p-5">
          <h2 className="mb-4 text-sm font-medium text-text-secondary">
            Latency Percentiles
          </h2>
          <ResponsiveContainer width="100%" height={280}>
            <AreaChart data={stats.latency_series}>
              <CartesianGrid strokeDasharray="3 3" stroke="#374151" />
              <XAxis
                dataKey="timestamp"
                tick={{ fill: '#9ca3af', fontSize: 12 }}
                tickFormatter={(v) => new Date(v).toLocaleTimeString()}
              />
              <YAxis
                tick={{ fill: '#9ca3af', fontSize: 12 }}
                label={{
                  value: 'ms',
                  position: 'insideLeft',
                  fill: '#9ca3af',
                }}
              />
              <Tooltip
                contentStyle={{
                  backgroundColor: '#1f2937',
                  border: '1px solid #374151',
                  borderRadius: 8,
                  color: '#f9fafb',
                }}
              />
              <Area
                type="monotone"
                dataKey="p99_ms"
                stroke="#ef4444"
                fill="#ef4444"
                fillOpacity={0.08}
                name="p99"
              />
              <Area
                type="monotone"
                dataKey="p95_ms"
                stroke="#f59e0b"
                fill="#f59e0b"
                fillOpacity={0.08}
                name="p95"
              />
              <Area
                type="monotone"
                dataKey="p50_ms"
                stroke="#10b981"
                fill="#10b981"
                fillOpacity={0.15}
                name="p50"
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      )}

      {/* Empty state when no analytics data */}
      {stats.throughput_series.length === 0 &&
        stats.latency_series.length === 0 && (
          <div className="rounded-xl border border-border bg-surface-light p-8 text-center text-text-secondary">
            <p className="text-sm">
              No analytics data yet. Send some traces to see charts here.
            </p>
          </div>
        )}
    </div>
  );
}
