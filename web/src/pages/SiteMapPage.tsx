import { useEffect, useState } from 'react';
import { PageHeader } from '../components/layout/PageHeader';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { Spinner } from '../components/ui/Spinner';
import { MethodBadge } from '../components/proxy/badges';
import { api, ApiError } from '../lib/api';
import { cn } from '../lib/cn';
import { formatCount } from '../lib/format';
import type { SitemapHost, SitemapPath } from '../lib/types';

interface PathEditor {
  host: string;
  path: string;
  note: string;
  tags: string;
}

function Chevron({ open }: { open: boolean }) {
  return (
    <svg
      width="12"
      height="12"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.4"
      strokeLinecap="round"
      strokeLinejoin="round"
      className={cn('shrink-0 text-zinc-500 transition-transform', open && 'rotate-90')}
      aria-hidden="true"
    >
      <path d="M9 6l6 6-6 6" />
    </svg>
  );
}

export function SiteMapPage() {
  const [hosts, setHosts] = useState<SitemapHost[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [editor, setEditor] = useState<PathEditor | null>(null);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      setHosts(await api.sitemap());
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to load site map');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const toggleHost = (host: string) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(host)) next.delete(host);
      else next.add(host);
      return next;
    });
  };

  const openEditor = (host: string, p: SitemapPath) => {
    setSaveError(null);
    setEditor({
      host,
      path: p.path,
      note: p.note ?? '',
      tags: (p.tags ?? []).join(', '),
    });
  };

  const saveNote = async () => {
    if (!editor) return;
    setSaving(true);
    setSaveError(null);
    const tags = editor.tags
      .split(',')
      .map((t) => t.trim())
      .filter(Boolean);
    try {
      await api.putSitemapNote({
        host: editor.host,
        path: editor.path,
        note: editor.note,
        tags,
      });
      // Reflect the saved note/tags in the tree without a full reload.
      setHosts((prev) =>
        prev.map((h) =>
          h.host !== editor.host
            ? h
            : {
                ...h,
                paths: h.paths.map((p) =>
                  p.path === editor.path ? { ...p, note: editor.note, tags } : p,
                ),
              },
        ),
      );
      setEditor(null);
    } catch (err) {
      setSaveError(err instanceof ApiError ? err.message : 'Failed to save note');
    } finally {
      setSaving(false);
    }
  };

  const exportJson = () => {
    const blob = new Blob([JSON.stringify(hosts, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = 'sitemap.json';
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader
        title="Site Map"
        subtitle="Discovered hosts and paths from captured traffic"
        actions={
          <Button size="sm" variant="outline" onClick={exportJson} disabled={hosts.length === 0}>
            Export JSON
          </Button>
        }
      />

      <div className="min-h-0 flex-1 overflow-auto">
        {loading ? (
          <div className="flex items-center gap-2 px-4 py-6 text-xs text-zinc-500">
            <Spinner /> Loading site map…
          </div>
        ) : error ? (
          <div className="px-4 py-6 text-xs text-red-400">{error}</div>
        ) : hosts.length === 0 ? (
          <div className="px-4 py-8 text-center text-xs text-zinc-400">No hosts discovered yet.</div>
        ) : (
          <div className="font-mono text-xs">
            {hosts.map((host) => {
              const open = expanded.has(host.host);
              return (
                <div key={host.host} className="border-b border-zinc-800/60">
                  <button
                    type="button"
                    onClick={() => toggleHost(host.host)}
                    className="flex w-full items-center gap-2 px-3 py-1.5 text-left hover:bg-zinc-800/40"
                  >
                    <Chevron open={open} />
                    <span className="flex-1 truncate text-zinc-200">{host.host}</span>
                    <span className="text-2xs text-zinc-500">{formatCount(host.count)} reqs</span>
                  </button>

                  {open && (
                    <div className="divide-y divide-zinc-800/40 border-t border-zinc-800/40 bg-canvas">
                      {host.paths.map((p) => (
                        <button
                          key={p.path}
                          type="button"
                          onClick={() => openEditor(host.host, p)}
                          className={cn(
                            'flex w-full items-center gap-2 py-1 pl-9 pr-3 text-left hover:bg-zinc-800/40',
                            !p.inScope && 'opacity-50',
                          )}
                          title={p.inScope ? p.path : `${p.path} (out of scope)`}
                        >
                          <span className="flex shrink-0 gap-1">
                            {p.methods.map((m) => (
                              <MethodBadge key={m} method={m} />
                            ))}
                          </span>
                          <span className="min-w-0 flex-1 truncate text-zinc-300">{p.path}</span>
                          {p.tags && p.tags.length > 0 && (
                            <span className="shrink-0 truncate text-2xs text-sky-400">
                              {p.tags.join(', ')}
                            </span>
                          )}
                          {p.note && (
                            <span className="shrink-0 text-2xs text-zinc-500" title={p.note}>
                              ✎
                            </span>
                          )}
                          <span className="shrink-0 text-2xs text-zinc-500 tabular-nums">
                            {formatCount(p.count)}
                          </span>
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* Note editor drawer */}
      {editor && (
        <div className="shrink-0 border-t border-zinc-800 bg-panel px-4 py-3">
          <div className="flex items-center justify-between pb-2">
            <span className="font-mono text-2xs text-zinc-400">
              <span className="text-zinc-500">{editor.host}</span>
              {editor.path}
            </span>
            <button
              type="button"
              title="Close"
              aria-label="Close note editor"
              onClick={() => setEditor(null)}
              className="rounded px-1 text-zinc-500 hover:text-zinc-200"
            >
              ✕
            </button>
          </div>
          <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
            <label className="flex flex-1 items-center gap-2 text-2xs text-zinc-500">
              Note
              <Input
                value={editor.note}
                placeholder="Annotation for this path"
                onChange={(e) => setEditor({ ...editor, note: e.target.value })}
                className="flex-1"
              />
            </label>
            <label className="flex flex-1 items-center gap-2 text-2xs text-zinc-500">
              Tags
              <Input
                value={editor.tags}
                placeholder="comma, separated"
                onChange={(e) => setEditor({ ...editor, tags: e.target.value })}
                className="flex-1 font-mono"
              />
            </label>
            <Button variant="primary" size="sm" disabled={saving} onClick={() => void saveNote()}>
              {saving ? 'Saving…' : 'Save'}
            </Button>
          </div>
          {saveError && <p className="pt-2 text-2xs text-red-400">{saveError}</p>}
        </div>
      )}
    </div>
  );
}
