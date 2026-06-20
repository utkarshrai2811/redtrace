import { memo } from 'react';
import { statusTextClass } from '../proxy/badges';
import type { CrawlURLView } from '../../lib/types';

interface UrlRowProps {
  url: CrawlURLView;
}

// Memoized so a live crawl appending one URL per frame only renders the new row
// — existing rows keep their props (stable url ref) and skip re-rendering,
// instead of rebuilding the whole table on every update.
const UrlRow = memo(function UrlRow({ url }: UrlRowProps) {
  return (
    <tr className="border-b border-zinc-800/50">
      <td className="truncate px-2 py-1.5 font-mono text-zinc-300" title={url.url}>
        {url.url}
      </td>
      <td className={`px-2 py-1.5 tabular-nums ${statusTextClass(url.statusCode)}`}>
        {url.statusCode > 0 ? url.statusCode : '–'}
      </td>
      <td className="truncate px-2 py-1.5 text-zinc-400" title={url.contentType}>
        {url.contentType || '—'}
      </td>
      <td className="px-2 py-1.5 text-right tabular-nums text-zinc-500">{url.depth}</td>
    </tr>
  );
});

interface UrlsTableProps {
  urls: CrawlURLView[];
}

export function UrlsTable({ urls }: UrlsTableProps) {
  if (urls.length === 0) {
    return (
      <div className="flex flex-1 items-center justify-center py-10 text-xs text-zinc-400">
        No URLs discovered yet. Start the crawl to populate this table.
      </div>
    );
  }

  return (
    <div className="min-h-0 flex-1 overflow-auto">
      <table className="w-full table-fixed border-collapse text-xs">
        <colgroup>
          <col />
          <col className="w-20" />
          <col className="w-48" />
          <col className="w-16" />
        </colgroup>
        <thead className="sticky top-0 z-10 bg-panel">
          <tr className="border-b border-zinc-800 text-left text-2xs uppercase tracking-wide text-zinc-500">
            <th className="px-2 py-1.5 font-medium">URL</th>
            <th className="px-2 py-1.5 font-medium">Status</th>
            <th className="px-2 py-1.5 font-medium">Type</th>
            <th className="px-2 py-1.5 text-right font-medium">Depth</th>
          </tr>
        </thead>
        <tbody>
          {urls.map((u) => (
            <UrlRow key={u.url} url={u} />
          ))}
        </tbody>
      </table>
    </div>
  );
}
