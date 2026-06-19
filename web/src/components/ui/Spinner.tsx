import { cn } from '../../lib/cn';

export function Spinner({ className }: { className?: string }) {
  return (
    <span
      className={cn(
        'inline-block h-3.5 w-3.5 animate-spin rounded-full border-2 border-zinc-600 border-t-accent',
        className,
      )}
      role="status"
      aria-label="Loading"
    />
  );
}
