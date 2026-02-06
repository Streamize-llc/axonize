import { Inbox } from 'lucide-react';

export function EmptyState({ message = 'No data found' }: { message?: string }) {
  return (
    <div className="flex flex-col items-center justify-center py-20 text-text-secondary">
      <Inbox className="h-8 w-8" />
      <p className="mt-3 text-sm">{message}</p>
    </div>
  );
}
