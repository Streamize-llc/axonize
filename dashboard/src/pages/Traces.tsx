import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useTraces } from '../hooks/useAPI';
import { Loading } from '../components/common/Loading';
import { ErrorState } from '../components/common/ErrorState';
import { EmptyState } from '../components/common/EmptyState';
import { StatusBadge } from '../components/common/StatusBadge';
import { formatDuration, formatTimestamp } from '../lib/format';
import type { TraceFilter } from '../lib/types';
import { ChevronLeft, ChevronRight, Search } from 'lucide-react';

const PAGE_SIZE = 25;

export function Traces() {
  const [filter, setFilter] = useState<TraceFilter>({
    limit: PAGE_SIZE,
    offset: 0,
  });
  const [serviceInput, setServiceInput] = useState('');
  const { data, isLoading, error, refetch } = useTraces(filter);

  const handleSearch = () => {
    setFilter((prev) => ({
      ...prev,
      service_name: serviceInput || undefined,
      offset: 0,
    }));
  };

  const traces = data?.traces ?? [];
  const total = data?.total ?? 0;
  const page = Math.floor((filter.offset ?? 0) / PAGE_SIZE) + 1;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">Traces</h1>

      {/* Filters */}
      <div className="flex items-center gap-3">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-text-secondary" />
          <input
            type="text"
            value={serviceInput}
            onChange={(e) => setServiceInput(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
            placeholder="Filter by service name..."
            className="w-full rounded-lg border border-border bg-surface-light py-2 pl-9 pr-3 text-sm text-text placeholder:text-text-secondary focus:border-primary focus:outline-none"
          />
        </div>
        <button
          onClick={handleSearch}
          className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-white hover:bg-primary-dark"
        >
          Search
        </button>
      </div>

      {/* Table */}
      {isLoading ? (
        <Loading message="Loading traces..." />
      ) : error ? (
        <ErrorState message={error.message} onRetry={refetch} />
      ) : traces.length === 0 ? (
        <EmptyState message="No traces found" />
      ) : (
        <div className="overflow-hidden rounded-xl border border-border">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-border bg-surface-light text-text-secondary">
              <tr>
                <th className="px-4 py-3 font-medium">Trace ID</th>
                <th className="px-4 py-3 font-medium">Service</th>
                <th className="px-4 py-3 font-medium">Time</th>
                <th className="px-4 py-3 font-medium text-right">Duration</th>
                <th className="px-4 py-3 font-medium text-right">Spans</th>
                <th className="px-4 py-3 font-medium text-center">Status</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {traces.map((t) => (
                <tr
                  key={t.trace_id}
                  className="bg-surface hover:bg-surface-light transition-colors"
                >
                  <td className="px-4 py-3">
                    <Link
                      to={`/traces/${t.trace_id}`}
                      className="font-mono text-xs text-primary hover:underline"
                    >
                      {t.trace_id.slice(0, 16)}...
                    </Link>
                  </td>
                  <td className="px-4 py-3 text-text-secondary">
                    {t.service_name}
                  </td>
                  <td className="px-4 py-3 text-text-secondary text-xs">
                    {formatTimestamp(t.start_time)}
                  </td>
                  <td className="px-4 py-3 text-right font-mono">
                    {formatDuration(t.duration_ms)}
                  </td>
                  <td className="px-4 py-3 text-right text-text-secondary">
                    {t.span_count}
                  </td>
                  <td className="px-4 py-3 text-center">
                    <StatusBadge
                      status={t.error_count > 0 ? 'error' : 'ok'}
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>

          {/* Pagination */}
          <div className="flex items-center justify-between border-t border-border bg-surface-light px-4 py-3 text-sm text-text-secondary">
            <span>
              {total} trace{total !== 1 ? 's' : ''} total
            </span>
            <div className="flex items-center gap-2">
              <button
                onClick={() =>
                  setFilter((f) => ({
                    ...f,
                    offset: Math.max(0, (f.offset ?? 0) - PAGE_SIZE),
                  }))
                }
                disabled={page <= 1}
                className="rounded p-1 hover:bg-surface-lighter disabled:opacity-30"
              >
                <ChevronLeft className="h-4 w-4" />
              </button>
              <span>
                Page {page} of {totalPages}
              </span>
              <button
                onClick={() =>
                  setFilter((f) => ({
                    ...f,
                    offset: (f.offset ?? 0) + PAGE_SIZE,
                  }))
                }
                disabled={page >= totalPages}
                className="rounded p-1 hover:bg-surface-lighter disabled:opacity-30"
              >
                <ChevronRight className="h-4 w-4" />
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
