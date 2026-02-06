import { useParams, Link } from 'react-router-dom';
import { ArrowLeft } from 'lucide-react';
import {
  ResponsiveContainer,
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
  Legend,
} from 'recharts';
import { useGPU, useGPUMetrics } from '../hooks/useAPI';
import { Loading } from '../components/common/Loading';
import { ErrorState } from '../components/common/ErrorState';
import { formatRelative } from '../lib/format';

export function GPUDetailPage() {
  const { uuid } = useParams<{ uuid: string }>();
  const { data: gpu, isLoading, error, refetch } = useGPU(uuid!);
  const { data: metricsData } = useGPUMetrics(uuid!);

  if (isLoading) return <Loading message="Loading GPU..." />;
  if (error) return <ErrorState message={error.message} onRetry={refetch} />;
  if (!gpu) return <ErrorState message="GPU not found" />;

  const metrics = (metricsData?.metrics ?? []).map((m) => ({
    ...m,
    time: new Date(m.timestamp).toLocaleTimeString(),
  }));

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center gap-3">
        <Link
          to="/gpus"
          className="rounded-lg p-1.5 text-text-secondary hover:bg-surface-lighter"
        >
          <ArrowLeft className="h-5 w-5" />
        </Link>
        <div>
          <h1 className="text-xl font-bold">{gpu.model || 'GPU Detail'}</h1>
          <p className="font-mono text-xs text-text-secondary">
            {gpu.resource_uuid}
          </p>
        </div>
      </div>

      {/* Info cards */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
        {[
          { label: 'Physical UUID', value: gpu.physical_uuid?.slice(0, 16) + '...' },
          { label: 'Resource Type', value: gpu.resource_type },
          { label: 'Node', value: gpu.node_id || '-' },
          { label: 'First Seen', value: gpu.first_seen ? formatRelative(gpu.first_seen) : '-' },
          { label: 'Last Seen', value: gpu.last_seen ? formatRelative(gpu.last_seen) : '-' },
        ].map(({ label, value }) => (
          <div
            key={label}
            className="rounded-lg border border-border bg-surface-light px-3 py-2"
          >
            <p className="text-xs text-text-secondary">{label}</p>
            <p className="mt-0.5 text-sm font-medium truncate">{value}</p>
          </div>
        ))}
      </div>

      {/* Metrics charts */}
      {metrics.length > 0 ? (
        <div className="space-y-4">
          {/* Utilization + Memory chart */}
          <div className="rounded-xl border border-border bg-surface-light p-5">
            <h2 className="mb-4 text-sm font-medium text-text-secondary">
              Utilization & Memory
            </h2>
            <ResponsiveContainer width="100%" height={300}>
              <LineChart data={metrics}>
                <CartesianGrid strokeDasharray="3 3" stroke="#374151" />
                <XAxis dataKey="time" tick={{ fill: '#9ca3af', fontSize: 11 }} />
                <YAxis
                  yAxisId="pct"
                  domain={[0, 100]}
                  tick={{ fill: '#9ca3af', fontSize: 11 }}
                  label={{ value: '%', position: 'insideLeft', fill: '#9ca3af' }}
                />
                <YAxis
                  yAxisId="gb"
                  orientation="right"
                  tick={{ fill: '#9ca3af', fontSize: 11 }}
                  label={{ value: 'GB', position: 'insideRight', fill: '#9ca3af' }}
                />
                <Tooltip
                  contentStyle={{
                    backgroundColor: '#1f2937',
                    border: '1px solid #374151',
                    borderRadius: 8,
                    color: '#f9fafb',
                  }}
                />
                <Legend />
                <Line
                  yAxisId="pct"
                  type="monotone"
                  dataKey="utilization"
                  stroke="#6366f1"
                  strokeWidth={2}
                  dot={false}
                  name="Utilization %"
                />
                <Line
                  yAxisId="gb"
                  type="monotone"
                  dataKey="memory_used_gb"
                  stroke="#10b981"
                  strokeWidth={2}
                  dot={false}
                  name="Memory (GB)"
                />
              </LineChart>
            </ResponsiveContainer>
          </div>

          {/* Power chart */}
          <div className="rounded-xl border border-border bg-surface-light p-5">
            <h2 className="mb-4 text-sm font-medium text-text-secondary">
              Power Consumption
            </h2>
            <ResponsiveContainer width="100%" height={200}>
              <LineChart data={metrics}>
                <CartesianGrid strokeDasharray="3 3" stroke="#374151" />
                <XAxis dataKey="time" tick={{ fill: '#9ca3af', fontSize: 11 }} />
                <YAxis
                  tick={{ fill: '#9ca3af', fontSize: 11 }}
                  label={{ value: 'W', position: 'insideLeft', fill: '#9ca3af' }}
                />
                <Tooltip
                  contentStyle={{
                    backgroundColor: '#1f2937',
                    border: '1px solid #374151',
                    borderRadius: 8,
                    color: '#f9fafb',
                  }}
                />
                <Line
                  type="monotone"
                  dataKey="power_watts"
                  stroke="#f59e0b"
                  strokeWidth={2}
                  dot={false}
                  name="Power (W)"
                />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </div>
      ) : (
        <div className="rounded-xl border border-border bg-surface-light p-8 text-center text-sm text-text-secondary">
          No metrics data available for this GPU.
        </div>
      )}
    </div>
  );
}
