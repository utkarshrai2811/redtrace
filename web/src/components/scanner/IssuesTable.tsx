import { memo } from 'react';
import { cn } from '../../lib/cn';
import { SeverityBadge, OriginBadge } from './badges';
import type { ScanIssueView } from '../../lib/types';

interface IssueRowProps {
  issue: ScanIssueView;
  selected: boolean;
  onSelect: (id: string) => void;
}

// Memoized so a live passive stream prepending one issue per frame only renders
// the new row — existing rows keep their props (stable issue ref, primitives)
// and skip re-rendering instead of rebuilding the whole table on every update.
const IssueRow = memo(function IssueRow({ issue, selected, onSelect }: IssueRowProps) {
  return (
    <tr
      onClick={() => onSelect(issue.id)}
      className={cn(
        'cursor-pointer border-b border-zinc-800/50',
        selected ? 'bg-accent/10' : 'hover:bg-zinc-800/40',
      )}
    >
      <td className="px-2 py-1.5">
        <SeverityBadge severity={issue.severity} />
      </td>
      <td className="truncate px-2 py-1.5 text-zinc-200" title={issue.name}>
        {issue.name}
      </td>
      <td
        className="truncate px-2 py-1.5 font-mono text-zinc-400"
        title={`${issue.host}${issue.path}`}
      >
        {issue.host}
        <span className="text-zinc-500">{issue.path}</span>
      </td>
      <td className="truncate px-2 py-1.5 font-mono text-zinc-400" title={issue.param ?? ''}>
        {issue.param || '—'}
      </td>
      <td className="px-2 py-1.5">
        <OriginBadge origin={issue.origin} />
      </td>
      <td className="px-2 py-1.5 capitalize text-zinc-400">{issue.confidence}</td>
    </tr>
  );
});

interface IssuesTableProps {
  issues: ScanIssueView[];
  selectedIssueId: string | null;
  onSelect: (id: string) => void;
}

export function IssuesTable({ issues, selectedIssueId, onSelect }: IssuesTableProps) {
  if (issues.length === 0) {
    return (
      <div className="flex flex-1 items-center justify-center py-10 text-xs text-zinc-400">
        No issues found yet. Run a scan or browse through the proxy to populate findings.
      </div>
    );
  }

  // issues arrive sorted by severity then recency from the store.
  return (
    <div className="min-h-0 flex-1 overflow-auto">
      <table className="w-full table-fixed border-collapse text-xs">
        <colgroup>
          <col className="w-20" />
          <col />
          <col className="w-[28%]" />
          <col className="w-32" />
          <col className="w-20" />
          <col className="w-20" />
        </colgroup>
        <thead className="sticky top-0 z-10 bg-panel">
          <tr className="border-b border-zinc-800 text-left text-2xs uppercase tracking-wide text-zinc-500">
            <th className="px-2 py-1.5 font-medium">Severity</th>
            <th className="px-2 py-1.5 font-medium">Issue</th>
            <th className="px-2 py-1.5 font-medium">Host / Path</th>
            <th className="px-2 py-1.5 font-medium">Param</th>
            <th className="px-2 py-1.5 font-medium">Origin</th>
            <th className="px-2 py-1.5 font-medium">Confidence</th>
          </tr>
        </thead>
        <tbody>
          {issues.map((issue) => (
            <IssueRow
              key={issue.id}
              issue={issue}
              selected={issue.id === selectedIssueId}
              onSelect={onSelect}
            />
          ))}
        </tbody>
      </table>
    </div>
  );
}
