import { Link } from 'react-router-dom';
import { Cpu } from 'lucide-react';
import { useGPUs } from '../hooks/useAPI';
import { Loading } from '../components/common/Loading';
import { ErrorState } from '../components/common/ErrorState';
import { EmptyState } from '../components/common/EmptyState';
import { formatPercent, formatGB } from '../lib/format';
import { cn } from '../lib/cn';

function utilizationColor(pct: number): string {
  if (pct >= 90) return 'text-error';
  if (pct >= 70) return 'text-warning';
  return 'text-success';
}

function barColor(pct: number): string {
  if (pct >= 90) return 'bg-error';
  if (pct >= 70) return 'bg-warning';
  return 'bg-success';
}

export function GPUs() {
  const { data, isLoading, error, refetch } = useGPUs();

  if (isLoading) return <Loading message="Loading GPUs..." />;
  if (error) return <ErrorState message={error.message} onRetry={refetch} />;

  const gpus = data?.gpus ?? [];

  if (gpus.length === 0) {
    return (
      <div className="space-y-4">
        <h1 className="text-2xl font-bold">GPUs</h1>
        <EmptyState message="No GPUs registered. Send spans with GPU attribution to see them here." />
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">GPUs</h1>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {gpus.map((gpu) => {
          const memPct =
            gpu.memory_total_gb > 0
              ? (gpu.memory_used_gb / gpu.memory_total_gb) * 100
              : 0;

          return (
            <Link
              key={gpu.resource_uuid}
              to={`/gpus/${gpu.resource_uuid}`}
              className="group rounded-xl border border-border bg-surface-light p-5 hover:border-primary/50 transition-colors"
            >
              {/* Header */}
              <div className="flex items-start justify-between">
                <div className="flex items-center gap-2">
                  <Cpu className="h-5 w-5 text-primary" />
                  <span className="text-sm font-semibold">{gpu.model || 'GPU'}</span>
                </div>
                <span className="rounded-full bg-surface-lighter px-2 py-0.5 text-xs text-text-secondary">
                  {gpu.resource_type}
                </span>
              </div>

              {/* ID */}
              <p className="mt-1 font-mono text-xs text-text-secondary truncate">
                {gpu.resource_uuid}
              </p>

              {/* Metrics */}
              <div className="mt-4 space-y-3">
                {/* Utilization */}
                <div>
                  <div className="flex justify-between text-xs">
                    <span className="text-text-secondary">Utilization</span>
                    <span className={cn('font-mono', utilizationColor(gpu.utilization))}>
                      {formatPercent(gpu.utilization)}
                    </span>
                  </div>
                  <div className="mt-1 h-1.5 rounded-full bg-surface-lighter">
                    <div
                      className={cn('h-full rounded-full transition-all', barColor(gpu.utilization))}
                      style={{ width: `${Math.min(100, gpu.utilization)}%` }}
                    />
                  </div>
                </div>

                {/* Memory */}
                <div>
                  <div className="flex justify-between text-xs">
                    <span className="text-text-secondary">Memory</span>
                    <span className="font-mono text-text-secondary">
                      {formatGB(gpu.memory_used_gb)} / {formatGB(gpu.memory_total_gb)}
                    </span>
                  </div>
                  <div className="mt-1 h-1.5 rounded-full bg-surface-lighter">
                    <div
                      className={cn('h-full rounded-full transition-all', barColor(memPct))}
                      style={{ width: `${Math.min(100, memPct)}%` }}
                    />
                  </div>
                </div>
              </div>

              {/* Footer */}
              <div className="mt-3 flex items-center justify-between text-xs text-text-secondary">
                <span>{gpu.node_id || 'unknown node'}</span>
              </div>
            </Link>
          );
        })}
      </div>
    </div>
  );
}
