import { useMemo } from 'react';
import { cn } from '../../lib/cn';
import { formatBytes, formatDuration } from '../../lib/format';
import { statusTextClass } from '../proxy/badges';
import type { IntruderResultView } from '../../lib/types';

/** Find the most-common value in a list (the mode), or undefined if empty. */
function modeOf(values: number[]): number | undefined {
  if (values.length === 0) return undefined;
  const counts = new Map<number, number>();
  let best = values[0];
  let bestCount = 0;
  for (const v of values) {
    const c = (counts.get(v) ?? 0) + 1;
    counts.set(v, c);
    if (c > bestCount) {
      bestCount = c;
      best = v;
    }
  }
  return best;
}

interface ResultsTableProps {
  results: IntruderResultView[];
  selectedResultId: string | null;
  onSelect: (id: string) => void;
}

export function ResultsTable({ results, selectedResultId, onSelect }: ResultsTableProps) {
  // Anomaly baselines: the modal length and the modal status. Rows that deviate
  // from either — or carry an error — are the "interesting" responses.
  const { modalLength, modalStatus } = useMemo(() => {
    const ok = results.filter((r) => !r.error);
    return {
      modalLength: modeOf(ok.map((r) => r.length)),
      modalStatus: modeOf(ok.map((r) => r.statusCode)),
    };
  }, [results]);

  const sorted = useMemo(
    () => [...results].sort((a, b) => a.index - b.index),
    [results],
  );

  if (results.length === 0) {
    return (
      <div className="flex flex-1 items-center justify-center py-10 text-xs text-zinc-400">
        No results yet. Start the attack to populate this table.
      </div>
    );
  }

  return (
    <div className="min-h-0 flex-1 overflow-auto">
      <table className="w-full table-fixed border-collapse font-mono text-xs">
        <colgroup>
          <col className="w-14" />
          <col />
          <col className="w-20" />
          <col className="w-20" />
          <col className="w-20" />
          <col className="w-10" />
        </colgroup>
        <thead className="sticky top-0 z-10 bg-panel">
          <tr className="border-b border-zinc-800 text-left text-2xs uppercase tracking-wide text-zinc-500">
            <th className="px-2 py-1.5 text-right font-medium">#</th>
            <th className="px-2 py-1.5 font-medium">Payload(s)</th>
            <th className="px-2 py-1.5 font-medium">Status</th>
            <th className="px-2 py-1.5 text-right font-medium">Length</th>
            <th className="px-2 py-1.5 text-right font-medium">Time</th>
            <th className="px-1 py-1.5 text-center font-medium" title="Anomaly">
              !
            </th>
          </tr>
        </thead>
        <tbody>
          {sorted.map((r) => {
            const hasError = Boolean(r.error);
            const lengthAnomaly = modalLength !== undefined && !hasError && r.length !== modalLength;
            const statusAnomaly =
              modalStatus !== undefined && !hasError && r.statusCode !== modalStatus;
            const interesting = hasError || lengthAnomaly || statusAnomaly;
            const payloads = r.payloads.join(' · ');
            return (
              <tr
                key={r.id}
                onClick={() => onSelect(r.id)}
                className={cn(
                  'cursor-pointer border-b border-zinc-800/50 border-l-2',
                  r.id === selectedResultId
                    ? 'bg-accent/10'
                    : interesting
                      ? 'bg-amber-500/5 hover:bg-amber-500/10'
                      : 'hover:bg-zinc-800/40',
                  interesting ? 'border-l-amber-500/60' : 'border-l-transparent',
                )}
              >
                <td className="px-2 py-1 text-right text-zinc-500 tabular-nums">{r.index}</td>
                <td className="truncate px-2 py-1 text-zinc-300" title={payloads}>
                  {payloads || '—'}
                </td>
                <td className={cn('px-2 py-1 tabular-nums', statusTextClass(r.statusCode))}>
                  {r.statusCode > 0 ? r.statusCode : '–'}
                </td>
                <td
                  className={cn(
                    'px-2 py-1 text-right tabular-nums',
                    lengthAnomaly ? 'text-amber-400' : 'text-zinc-400',
                  )}
                >
                  {formatBytes(r.length)}
                </td>
                <td className="px-2 py-1 text-right text-zinc-500 tabular-nums">
                  {formatDuration(r.durationMs)}
                </td>
                <td className="px-1 py-1 text-center">
                  {hasError ? (
                    <span className="text-red-400" title={r.error}>
                      ✕
                    </span>
                  ) : interesting ? (
                    <span
                      className="text-amber-400"
                      title="Response differs from the common baseline"
                    >
                      ●
                    </span>
                  ) : null}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
