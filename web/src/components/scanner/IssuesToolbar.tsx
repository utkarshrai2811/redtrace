import { cn } from '../../lib/cn';
import { Input } from '../ui/Input';
import { Button } from '../ui/Button';
import { useScannerStore } from '../../store/scannerStore';
import type { Severity } from '../../lib/types';

const SEVERITIES: Severity[] = ['high', 'medium', 'low', 'info'];

// Chip colors mirror the severity badge palette so the filter reads the same as
// the table: high = red, medium/low = amber, info = blue.
const CHIP_ACTIVE: Record<Severity, string> = {
  high: 'bg-red-500/15 text-red-400 border-red-500/40',
  medium: 'bg-amber-500/15 text-amber-400 border-amber-500/40',
  low: 'bg-amber-500/10 text-amber-300 border-amber-500/30',
  info: 'bg-sky-500/15 text-sky-400 border-sky-500/40',
};

export function IssuesToolbar({ count }: { count: number }) {
  const severityFilter = useScannerStore((s) => s.severityFilter);
  const hostFilter = useScannerStore((s) => s.hostFilter);
  const toggleSeverity = useScannerStore((s) => s.toggleSeverity);
  const setHostFilter = useScannerStore((s) => s.setHostFilter);
  const clearIssues = useScannerStore((s) => s.clearIssues);

  return (
    <div className="flex shrink-0 flex-wrap items-center gap-2 border-b border-zinc-800 bg-panel/40 px-3 py-2">
      <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">
        Severity
      </span>
      <div className="flex items-center gap-1">
        {SEVERITIES.map((sev) => {
          const active = severityFilter.has(sev);
          return (
            <button
              key={sev}
              type="button"
              aria-pressed={active}
              onClick={() => toggleSeverity(sev)}
              className={cn(
                'rounded border px-2 py-0.5 text-2xs font-medium uppercase tracking-wide transition-colors',
                'focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent/60',
                active
                  ? CHIP_ACTIVE[sev]
                  : 'border-zinc-700 text-zinc-500 hover:border-zinc-600 hover:text-zinc-300',
              )}
            >
              {sev}
            </button>
          );
        })}
      </div>

      <Input
        aria-label="Host filter"
        placeholder="Filter by host…"
        value={hostFilter}
        onChange={(e) => setHostFilter(e.target.value)}
        className="w-48 font-mono"
      />

      <div className="ml-auto flex items-center gap-2">
        <span className="text-2xs text-zinc-500">
          {count.toLocaleString('en-US')} issue{count === 1 ? '' : 's'}
        </span>
        <Button
          size="sm"
          variant="ghost"
          onClick={() => {
            if (window.confirm('Clear all findings? This cannot be undone.')) {
              void clearIssues();
            }
          }}
          title="Clear all findings"
        >
          <svg
            width="13"
            height="13"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden="true"
          >
            <path d="M3 6h18M8 6V4h8v2M6 6l1 14h10l1-14" />
          </svg>
          Clear
        </Button>
      </div>
    </div>
  );
}
