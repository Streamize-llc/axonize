import { useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { ArrowLeft } from 'lucide-react';
import { useTrace } from '../hooks/useAPI';
import { Loading } from '../components/common/Loading';
import { ErrorState } from '../components/common/ErrorState';
import { StatusBadge } from '../components/common/StatusBadge';
import { formatDuration, formatTimestamp } from '../lib/format';
import { cn } from '../lib/cn';
import type { SpanDetail } from '../lib/types';

// ---- Gantt timeline ----

function GanttBar({
  span,
  traceStart,
  traceDuration,
  depth,
  selected,
  onSelect,
}: {
  span: SpanDetail;
  traceStart: number;
  traceDuration: number;
  depth: number;
  selected: boolean;
  onSelect: (span: SpanDetail) => void;
}) {
  const spanStart = new Date(span.start_time).getTime();
  const leftPct = ((spanStart - traceStart) / traceDuration) * 100;
  const widthPct = Math.max(0.5, (span.duration_ms / traceDuration) * 100);

  return (
    <>
      <button
        onClick={() => onSelect(span)}
        className={cn(
          'flex w-full items-center gap-2 border-b border-border px-3 py-1.5 text-left hover:bg-surface-light transition-colors',
          selected && 'bg-primary/10',
        )}
      >
        <div
          className="shrink-0 truncate text-xs text-text-secondary"
          style={{ paddingLeft: depth * 16, width: 200 }}
        >
          {span.name}
        </div>
        <div className="relative flex-1 h-5">
          <div
            className={cn(
              'absolute top-0.5 h-4 rounded-sm',
              span.status === 'error' ? 'bg-error/70' : 'bg-primary/70',
            )}
            style={{
              left: `${leftPct}%`,
              width: `${widthPct}%`,
              minWidth: 4,
            }}
          />
        </div>
        <span className="shrink-0 w-20 text-right font-mono text-xs text-text-secondary">
          {formatDuration(span.duration_ms)}
        </span>
      </button>
      {span.children.map((child) => (
        <GanttBar
          key={child.span_id}
          span={child}
          traceStart={traceStart}
          traceDuration={traceDuration}
          depth={depth + 1}
          selected={selected}
          onSelect={onSelect}
        />
      ))}
    </>
  );
}

// ---- Span detail panel ----

function SpanPanel({ span }: { span: SpanDetail }) {
  return (
    <div className="rounded-xl border border-border bg-surface-light p-4 space-y-4">
      <div>
        <h3 className="text-sm font-semibold">{span.name}</h3>
        <p className="text-xs text-text-secondary">
          {span.span_id}
        </p>
      </div>

      <div className="grid grid-cols-2 gap-3 text-sm">
        <div>
          <p className="text-text-secondary text-xs">Status</p>
          <StatusBadge status={span.status} />
        </div>
        <div>
          <p className="text-text-secondary text-xs">Duration</p>
          <p className="font-mono">{formatDuration(span.duration_ms)}</p>
        </div>
        <div>
          <p className="text-text-secondary text-xs">Start</p>
          <p className="text-xs">{formatTimestamp(span.start_time)}</p>
        </div>
        <div>
          <p className="text-text-secondary text-xs">End</p>
          <p className="text-xs">{formatTimestamp(span.end_time)}</p>
        </div>
      </div>

      {span.error_message && (
        <div className="rounded-lg bg-error/10 p-3">
          <p className="text-xs font-medium text-error">Error</p>
          <p className="mt-1 text-xs text-error/80">{span.error_message}</p>
        </div>
      )}

      {span.attributes && Object.keys(span.attributes).length > 0 && (
        <div>
          <p className="text-xs font-medium text-text-secondary mb-2">
            Attributes
          </p>
          <div className="space-y-1">
            {Object.entries(span.attributes).map(([k, v]) => (
              <div key={k} className="flex text-xs">
                <span className="w-40 shrink-0 text-text-secondary truncate">
                  {k}
                </span>
                <span className="font-mono truncate">{v}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

// ---- Page ----

export function TraceDetailPage() {
  const { traceId } = useParams<{ traceId: string }>();
  const { data: trace, isLoading, error, refetch } = useTrace(traceId!);
  const [selectedSpan, setSelectedSpan] = useState<SpanDetail | null>(null);

  if (isLoading) return <Loading message="Loading trace..." />;
  if (error) return <ErrorState message={error.message} onRetry={refetch} />;
  if (!trace) return <ErrorState message="Trace not found" />;

  const traceStart = new Date(trace.start_time).getTime();
  const traceDuration = trace.duration_ms;

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center gap-3">
        <Link
          to="/traces"
          className="rounded-lg p-1.5 text-text-secondary hover:bg-surface-lighter"
        >
          <ArrowLeft className="h-5 w-5" />
        </Link>
        <div>
          <h1 className="text-xl font-bold">Trace Detail</h1>
          <p className="font-mono text-xs text-text-secondary">{trace.trace_id}</p>
        </div>
      </div>

      {/* Summary */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-6">
        {[
          { label: 'Service', value: trace.service_name },
          { label: 'Environment', value: trace.environment || '-' },
          { label: 'Duration', value: formatDuration(trace.duration_ms) },
          { label: 'Spans', value: String(trace.span_count) },
          { label: 'Errors', value: String(trace.error_count) },
          { label: 'Time', value: formatTimestamp(trace.start_time) },
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

      {/* Gantt + Detail */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
        <div className="lg:col-span-2 rounded-xl border border-border bg-surface overflow-hidden">
          <div className="border-b border-border bg-surface-light px-4 py-2 text-xs font-medium text-text-secondary">
            Span Timeline
          </div>
          <div className="max-h-[500px] overflow-y-auto">
            {trace.spans.map((span) => (
              <GanttBar
                key={span.span_id}
                span={span}
                traceStart={traceStart}
                traceDuration={traceDuration}
                depth={0}
                selected={selectedSpan?.span_id === span.span_id}
                onSelect={setSelectedSpan}
              />
            ))}
          </div>
        </div>

        <div>
          {selectedSpan ? (
            <SpanPanel span={selectedSpan} />
          ) : (
            <div className="rounded-xl border border-border bg-surface-light p-6 text-center text-sm text-text-secondary">
              Click a span to see details
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
