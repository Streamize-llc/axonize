import { AlertTriangle } from 'lucide-react';

export function ErrorState({
  message = 'Something went wrong',
  onRetry,
}: {
  message?: string;
  onRetry?: () => void;
}) {
  return (
    <div className="flex flex-col items-center justify-center py-20 text-text-secondary">
      <AlertTriangle className="h-8 w-8 text-error" />
      <p className="mt-3 text-sm">{message}</p>
      {onRetry && (
        <button
          onClick={onRetry}
          className="mt-3 rounded-md bg-surface-lighter px-4 py-1.5 text-sm text-text hover:bg-border"
        >
          Retry
        </button>
      )}
    </div>
  );
}
