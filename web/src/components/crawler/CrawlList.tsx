import { useState } from 'react';
import { Button } from '../ui/Button';
import { Input } from '../ui/Input';
import { Spinner } from '../ui/Spinner';
import { cn } from '../../lib/cn';
import { StatusBadge } from './badges';
import { useCrawlerStore } from '../../store/crawlerStore';

function NewCrawlForm({ onClose }: { onClose: () => void }) {
  const createTask = useCrawlerStore((s) => s.createTask);
  const createError = useCrawlerStore((s) => s.createError);
  const [seed, setSeed] = useState('');
  const [maxDepth, setMaxDepth] = useState(3);
  const [maxPages, setMaxPages] = useState(100);
  const [submitting, setSubmitting] = useState(false);

  const trimmed = seed.trim();
  const canCreate = trimmed.length > 0 && !submitting;

  const submit = async () => {
    if (!canCreate) return;
    setSubmitting(true);
    // Keep the form open on failure so the bad seed shows its error inline and
    // can be corrected, rather than closing and clobbering the list.
    const ok = await createTask({ name: '', seed: trimmed, maxDepth, maxPages });
    setSubmitting(false);
    if (ok) onClose();
  };

  return (
    <form
      className="space-y-2 border-b border-zinc-800 px-3 py-3"
      onSubmit={(e) => {
        e.preventDefault();
        void submit();
      }}
    >
      <label className="block">
        <span className="mb-1 block text-2xs font-medium uppercase tracking-wider text-zinc-500">
          Seed URL
        </span>
        <Input
          value={seed}
          placeholder="https://example.com/"
          aria-label="Seed URL"
          autoFocus
          onChange={(e) => setSeed(e.target.value)}
          className="w-full font-mono"
        />
      </label>
      <div className="flex items-center gap-2">
        <label className="flex flex-1 items-center justify-between gap-1.5 text-xs text-zinc-400">
          <span>Max depth</span>
          <Input
            type="number"
            min={1}
            value={maxDepth}
            aria-label="Max depth"
            onChange={(e) => setMaxDepth(Math.max(1, Number(e.target.value) || 1))}
            className="w-16 tabular-nums"
          />
        </label>
        <label className="flex flex-1 items-center justify-between gap-1.5 text-xs text-zinc-400">
          <span>Max pages</span>
          <Input
            type="number"
            min={1}
            value={maxPages}
            aria-label="Max pages"
            onChange={(e) => setMaxPages(Math.max(1, Number(e.target.value) || 1))}
            className="w-16 tabular-nums"
          />
        </label>
      </div>
      {createError && <p className="text-xs text-red-400">{createError}</p>}
      <div className="flex items-center justify-end gap-2">
        <Button size="sm" variant="ghost" onClick={onClose}>
          Cancel
        </Button>
        <Button size="sm" variant="primary" type="submit" disabled={!canCreate}>
          Create
        </Button>
      </div>
    </form>
  );
}

export function CrawlList() {
  const tasks = useCrawlerStore((s) => s.tasks);
  const loading = useCrawlerStore((s) => s.loadingTasks);
  const error = useCrawlerStore((s) => s.tasksError);
  const selectedTaskId = useCrawlerStore((s) => s.selectedTaskId);
  const selectTask = useCrawlerStore((s) => s.selectTask);
  const deleteTask = useCrawlerStore((s) => s.deleteTask);

  const [showForm, setShowForm] = useState(false);

  return (
    <div className="flex w-[240px] shrink-0 flex-col border-r border-zinc-800 bg-panel">
      <div className="flex shrink-0 items-center justify-between border-b border-zinc-800 px-3 py-2">
        <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">Crawls</span>
        <Button size="sm" variant="outline" onClick={() => setShowForm((v) => !v)}>
          + New crawl
        </Button>
      </div>

      {showForm && <NewCrawlForm onClose={() => setShowForm(false)} />}

      <div className="min-h-0 flex-1 overflow-auto p-1.5">
        {loading && tasks.length === 0 ? (
          <div className="flex items-center gap-2 px-2 py-3 text-xs text-zinc-500">
            <Spinner /> Loading…
          </div>
        ) : error ? (
          <p className="px-2 py-3 text-xs text-red-400">{error}</p>
        ) : tasks.length === 0 ? (
          <p className="px-2 py-3 text-xs text-zinc-400">
            No crawls yet. Start one above or send a request from the proxy.
          </p>
        ) : (
          <div className="space-y-0.5">
            {tasks.map((task) => {
              const running = task.status === 'running';
              return (
                <div
                  key={task.id}
                  className={cn(
                    'group flex items-center gap-1.5 rounded-md px-2 py-1.5 text-sm transition-colors',
                    task.id === selectedTaskId
                      ? 'bg-zinc-800/80 text-zinc-100'
                      : 'text-zinc-400 hover:bg-zinc-800/50 hover:text-zinc-200',
                  )}
                >
                  <button
                    type="button"
                    onClick={() => void selectTask(task.id)}
                    className="min-w-0 flex-1 text-left"
                    title={task.seed || task.name}
                  >
                    <div className="truncate">{task.name || task.host || 'Untitled'}</div>
                    <div className="flex items-center gap-1.5">
                      <span className="truncate font-mono text-2xs text-zinc-500">
                        {task.host || 'no host'}
                      </span>
                      <span className="flex shrink-0 items-center gap-1 font-mono text-2xs tabular-nums text-zinc-500">
                        {running && <Spinner className="h-2.5 w-2.5" />}
                        {task.pages}/{task.found}
                      </span>
                    </div>
                  </button>
                  {!running && <StatusBadge status={task.status} />}
                  <button
                    type="button"
                    title="Delete crawl"
                    aria-label="Delete crawl"
                    onClick={() => {
                      const msg = running
                        ? 'This crawl is running and will be stopped. Delete it?'
                        : 'Delete this crawl?';
                      if (window.confirm(msg)) void deleteTask(task.id);
                    }}
                    className="shrink-0 rounded px-1 text-zinc-600 opacity-0 transition-opacity hover:text-red-400 group-hover:opacity-100 group-focus-within:opacity-100 focus-visible:opacity-100"
                  >
                    ✕
                  </button>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
