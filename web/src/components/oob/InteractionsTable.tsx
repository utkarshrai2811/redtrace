import { memo } from 'react';
import { cn } from '../../lib/cn';
import { formatTime } from '../../lib/format';
import { ProtocolBadge } from './badges';
import type { OOBInteractionView } from '../../lib/types';

interface InteractionRowProps {
  interaction: OOBInteractionView;
  selected: boolean;
  onSelect: (id: string) => void;
}

// Memoized so a live OOB stream prepending one interaction per frame only
// renders the new row — existing rows keep their props (stable interaction ref,
// primitives) and skip re-rendering instead of rebuilding the whole table.
const InteractionRow = memo(function InteractionRow({
  interaction,
  selected,
  onSelect,
}: InteractionRowProps) {
  return (
    <tr
      onClick={() => onSelect(interaction.id)}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          onSelect(interaction.id);
        }
      }}
      tabIndex={0}
      role="button"
      aria-label={`${interaction.protocol} interaction from ${interaction.sourceIp}`}
      className={cn(
        'cursor-pointer border-b border-zinc-800/50 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-inset focus-visible:ring-accent/60',
        selected ? 'bg-accent/10' : 'hover:bg-zinc-800/40',
      )}
    >
      <td className="px-2 py-1.5">
        <ProtocolBadge protocol={interaction.protocol} />
      </td>
      <td className="truncate px-2 py-1.5 font-mono text-zinc-300" title={interaction.sourceIp}>
        {interaction.sourceIp || '—'}
      </td>
      <td className="truncate px-2 py-1.5 font-mono text-zinc-400" title={interaction.query}>
        {interaction.query || '—'}
      </td>
      <td className="truncate px-2 py-1.5 text-zinc-400" title={interaction.detail}>
        {interaction.detail || '—'}
      </td>
      <td className="px-2 py-1.5 font-mono tabular-nums text-zinc-500">
        {formatTime(interaction.createdAt)}
      </td>
    </tr>
  );
});

interface InteractionsTableProps {
  interactions: OOBInteractionView[];
  selectedId: string | null;
  onSelect: (id: string) => void;
}

export function InteractionsTable({ interactions, selectedId, onSelect }: InteractionsTableProps) {
  if (interactions.length === 0) {
    return (
      <div className="flex flex-1 items-center justify-center py-10 text-xs text-zinc-400">
        No interactions yet. Plant a generated payload and any DNS/HTTP/HTTPS callback to it appears
        here live.
      </div>
    );
  }

  // interactions arrive newest-first from the store.
  return (
    <div className="min-h-0 flex-1 overflow-auto">
      <table className="w-full table-fixed border-collapse text-xs">
        <colgroup>
          <col className="w-16" />
          <col className="w-40" />
          <col className="w-[30%]" />
          <col />
          <col className="w-20" />
        </colgroup>
        <thead className="sticky top-0 z-10 bg-panel">
          <tr className="border-b border-zinc-800 text-left text-2xs uppercase tracking-wide text-zinc-500">
            <th className="px-2 py-1.5 font-medium">Protocol</th>
            <th className="px-2 py-1.5 font-medium">Source IP</th>
            <th className="px-2 py-1.5 font-medium">Query</th>
            <th className="px-2 py-1.5 font-medium">Detail</th>
            <th className="px-2 py-1.5 font-medium">Time</th>
          </tr>
        </thead>
        <tbody>
          {interactions.map((i) => (
            <InteractionRow
              key={i.id}
              interaction={i}
              selected={i.id === selectedId}
              onSelect={onSelect}
            />
          ))}
        </tbody>
      </table>
    </div>
  );
}
