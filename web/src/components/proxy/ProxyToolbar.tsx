import { useEffect, useRef, useState } from 'react';
import { Select } from '../ui/Select';
import { Input } from '../ui/Input';
import { Toggle } from '../ui/Toggle';
import { Button } from '../ui/Button';
import { Badge } from '../ui/Badge';
import { useProxyStore } from '../../store/proxyStore';
import { useDebouncedCallback } from '../../hooks/useDebouncedCallback';

const METHODS = ['ANY', 'GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'HEAD', 'OPTIONS'];
const STATUSES = ['ANY', '2xx', '3xx', '4xx', '5xx'];

export function ProxyToolbar() {
  const filters = useProxyStore((s) => s.filters);
  const hosts = useProxyStore((s) => s.hosts);
  const setFilter = useProxyStore((s) => s.setFilter);
  const fetchList = useProxyStore((s) => s.fetchList);
  const clearRows = useProxyStore((s) => s.clearRows);
  const total = useProxyStore((s) => s.total);

  const intercept = useProxyStore((s) => s.intercept);
  const setInterceptEnabled = useProxyStore((s) => s.setInterceptEnabled);
  const setInterceptResponses = useProxyStore((s) => s.setInterceptResponses);

  // Local mirrors of the text inputs so typing feels instant; the store is
  // updated on a debounce.
  const [q, setQ] = useState(filters.q);
  const [mime, setMime] = useState(filters.mime);

  // Keep local state in sync if filters are reset elsewhere.
  const lastFilters = useRef({ q: filters.q, mime: filters.mime });
  useEffect(() => {
    if (filters.q !== lastFilters.current.q) setQ(filters.q);
    if (filters.mime !== lastFilters.current.mime) setMime(filters.mime);
    lastFilters.current = { q: filters.q, mime: filters.mime };
  }, [filters.q, filters.mime]);

  const debouncedQ = useDebouncedCallback((value: string) => setFilter('q', value), 300);
  const debouncedMime = useDebouncedCallback((value: string) => setFilter('mime', value), 300);

  return (
    <div className="shrink-0 border-b border-zinc-800 bg-panel/40">
      {/* Intercept controls */}
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2 px-4 py-2">
        <div className="flex items-center gap-2">
          <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">
            Intercept
          </span>
          <Toggle
            checked={intercept.enabled}
            onChange={(v) => void setInterceptEnabled(v)}
            label={intercept.enabled ? 'On' : 'Off'}
          />
          {intercept.count > 0 && <Badge tone="accent">{intercept.count} held</Badge>}
        </div>

        <label className="flex cursor-pointer items-center gap-1.5 text-xs text-zinc-400">
          <input
            type="checkbox"
            checked={intercept.interceptResponses}
            onChange={(e) => void setInterceptResponses(e.target.checked)}
            className="h-3.5 w-3.5 rounded border-zinc-600 bg-zinc-900 text-accent focus:ring-0 focus:ring-offset-0"
          />
          Responses too
        </label>

        <div className="ml-auto flex items-center gap-2">
          <span className="text-2xs text-zinc-500">{total.toLocaleString('en-US')} captured</span>
          <Button size="sm" variant="ghost" onClick={() => void fetchList()} title="Refresh list">
            <svg
              width="13"
              height="13"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d="M21 12a9 9 0 11-2.6-6.4M21 3v6h-6" />
            </svg>
            Refresh
          </Button>
          <Button
            size="sm"
            variant="ghost"
            onClick={() => {
              if (window.confirm('Clear all captured traffic? This cannot be undone.')) {
                void clearRows();
              }
            }}
            title="Clear all captured traffic"
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
            >
              <path d="M3 6h18M8 6V4h8v2M6 6l1 14h10l1-14" />
            </svg>
            Clear
          </Button>
        </div>
      </div>

      {/* Filter bar */}
      <div className="flex flex-wrap items-center gap-2 border-t border-zinc-800/60 px-4 py-2">
        <Select
          aria-label="Method filter"
          value={filters.method}
          onChange={(e) => setFilter('method', e.target.value)}
        >
          {METHODS.map((m) => (
            <option key={m} value={m}>
              {m === 'ANY' ? 'Method: ANY' : m}
            </option>
          ))}
        </Select>

        <Select
          aria-label="Host filter"
          value={filters.host}
          onChange={(e) => setFilter('host', e.target.value)}
          className="max-w-[220px]"
        >
          <option value="ANY">Host: ANY</option>
          {hosts.map((h) => (
            <option key={h} value={h}>
              {h}
            </option>
          ))}
        </Select>

        <Select
          aria-label="Status filter"
          value={filters.status}
          onChange={(e) => setFilter('status', e.target.value)}
        >
          {STATUSES.map((s) => (
            <option key={s} value={s}>
              {s === 'ANY' ? 'Status: ANY' : s}
            </option>
          ))}
        </Select>

        <Input
          aria-label="MIME filter"
          placeholder="MIME"
          value={mime}
          onChange={(e) => {
            setMime(e.target.value);
            debouncedMime(e.target.value);
          }}
          className="w-28"
        />

        <Input
          aria-label="Search"
          placeholder="Search URL / body…"
          value={q}
          onChange={(e) => {
            setQ(e.target.value);
            debouncedQ(e.target.value);
          }}
          className="w-52"
        />

        <Toggle
          tone="accent"
          checked={filters.scope}
          onChange={(v) => setFilter('scope', v)}
          label="In scope only"
        />
      </div>
    </div>
  );
}
