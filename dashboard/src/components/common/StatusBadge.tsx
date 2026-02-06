import { cn } from '../../lib/cn';

export function StatusBadge({ status }: { status: string }) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium',
        status === 'error'
          ? 'bg-error/15 text-error'
          : status === 'ok'
            ? 'bg-success/15 text-success'
            : 'bg-surface-lighter text-text-secondary',
      )}
    >
      {status}
    </span>
  );
}
