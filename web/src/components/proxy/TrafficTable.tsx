import { memo } from 'react';
import { useProxyStore } from '../../store/proxyStore';
import { MethodBadge, statusTextClass } from './badges';
import { formatBytes, formatDuration, formatTime } from '../../lib/format';
import { Spinner } from '../ui/Spinner';
import { cn } from '../../lib/cn';
import type { RequestSummary } from '../../lib/types';

// Memoized so that streaming a new row (prepended) or selecting a row only
// re-renders the affected rows, not the whole table. The row number comes from
// a CSS counter (see .traffic-num) so it carries no per-row index prop.
const TrafficRow = memo(function TrafficRow({
  row,
  selected,
  onSelect,
  onDelete,
}: {
  row: RequestSummary;
  selected: boolean;
  onSelect: (id: string) => void;
  onDelete: (id: string) => void;
}) {
  return (
    <tr
      onClick={() => onSelect(row.id)}
      className={cn(
        'group cursor-pointer border-b border-zinc-800/50',
        selected ? 'bg-accent/10' : 'hover:bg-zinc-800/40',
        !row.inScope && 'opacity-50',
      )}
    >
      <td className="px-2 py-1 text-right text-zinc-600 tabular-nums">
        <span className="traffic-num" />
      </td>
      <td className="px-2 py-1">
        <MethodBadge method={row.method} />
      </td>
      <td className="max-w-[180px] truncate px-2 py-1 text-zinc-300" title={row.host}>
        {row.host}
      </td>
      <td className="max-w-[360px] truncate px-2 py-1 text-zinc-400" title={row.path}>
        {row.path || '/'}
      </td>
      <td className={cn('px-2 py-1 tabular-nums', statusTextClass(row.statusCode))}>
        {row.hasResponse && row.statusCode > 0 ? row.statusCode : '–'}
      </td>
      <td className="px-2 py-1 text-right text-zinc-400 tabular-nums">
        {formatBytes(row.responseLength)}
      </td>
      <td className="max-w-[140px] truncate px-2 py-1 text-zinc-500" title={row.mimeType}>
        {row.mimeType || '–'}
      </td>
      <td className="px-2 py-1 text-zinc-500 tabular-nums">{formatTime(row.timestamp)}</td>
      <td className="px-2 py-1 text-right text-zinc-500 tabular-nums">
        {formatDuration(row.durationMs)}
      </td>
      <td className="px-1 py-1 text-right">
        <button
          type="button"
          title="Delete"
          onClick={(e) => {
            e.stopPropagation();
            onDelete(row.id);
          }}
          className="invisible rounded px-1 text-zinc-500 hover:text-red-400 group-hover:visible"
        >
          ✕
        </button>
      </td>
    </tr>
  );
});

export function TrafficTable() {
  const rows = useProxyStore((s) => s.rows);
  const loading = useProxyStore((s) => s.loadingList);
  const error = useProxyStore((s) => s.listError);
  const selectedId = useProxyStore((s) => s.selectedId);
  const selectRow = useProxyStore((s) => s.selectRow);
  const deleteRow = useProxyStore((s) => s.deleteRow);

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      {error && (
        <div className="border-b border-red-500/30 bg-red-500/10 px-4 py-1.5 text-xs text-red-400">
          {error}
        </div>
      )}
      <div className="min-h-0 flex-1 overflow-auto">
        <table className="w-full border-collapse font-mono text-xs">
          <thead className="sticky top-0 z-10 bg-panel">
            <tr className="border-b border-zinc-800 text-left text-2xs uppercase tracking-wide text-zinc-500">
              <th className="px-2 py-1.5 text-right font-medium">#</th>
              <th className="px-2 py-1.5 font-medium">Method</th>
              <th className="px-2 py-1.5 font-medium">Host</th>
              <th className="px-2 py-1.5 font-medium">Path</th>
              <th className="px-2 py-1.5 font-medium">Status</th>
              <th className="px-2 py-1.5 text-right font-medium">Length</th>
              <th className="px-2 py-1.5 font-medium">MIME</th>
              <th className="px-2 py-1.5 font-medium">Time</th>
              <th className="px-2 py-1.5 text-right font-medium">Dur</th>
              <th className="px-1 py-1.5" />
            </tr>
          </thead>
          <tbody className="traffic-rows">
            {rows.map((row) => (
              <TrafficRow
                key={row.id}
                row={row}
                selected={row.id === selectedId}
                onSelect={selectRow}
                onDelete={deleteRow}
              />
            ))}
          </tbody>
        </table>

        {rows.length === 0 && !loading && (
          <div className="flex h-full min-h-[160px] flex-col items-center justify-center gap-1 py-12 text-center text-zinc-600">
            <span className="text-sm">No traffic captured</span>
            <span className="text-2xs">
              Point your browser at the proxy and requests will stream in live.
            </span>
          </div>
        )}

        {loading && rows.length === 0 && (
          <div className="flex items-center justify-center gap-2 py-12 text-xs text-zinc-500">
            <Spinner /> Loading traffic…
          </div>
        )}
      </div>
    </div>
  );
}
