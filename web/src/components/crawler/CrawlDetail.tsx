import { Button } from '../ui/Button';
import { Spinner } from '../ui/Spinner';
import { StatusBadge } from './badges';
import { UrlsTable } from './UrlsTable';
import { useCrawlerStore } from '../../store/crawlerStore';

export function CrawlDetail() {
  const selectedTaskId = useCrawlerStore((s) => s.selectedTaskId);
  const task = useCrawlerStore((s) => s.selectedTask);
  const taskUrls = useCrawlerStore((s) => s.taskUrls);
  const loading = useCrawlerStore((s) => s.loadingDetail);
  const detailError = useCrawlerStore((s) => s.detailError);
  const actionError = useCrawlerStore((s) => s.actionError);
  const startTask = useCrawlerStore((s) => s.startTask);
  const stopTask = useCrawlerStore((s) => s.stopTask);

  if (!selectedTaskId) {
    return (
      <div className="flex h-full flex-1 items-center justify-center text-xs text-zinc-400">
        Select a crawl to view its progress and discovered URLs.
      </div>
    );
  }

  if (loading) {
    return (
      <div className="flex h-full flex-1 items-center justify-center gap-2 text-xs text-zinc-500">
        <Spinner /> Loading crawl…
      </div>
    );
  }

  if (detailError) {
    return (
      <div className="flex h-full flex-1 items-center justify-center text-xs text-red-400">
        {detailError}
      </div>
    );
  }

  if (!task) return null;

  const running = task.status === 'running';

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      {/* Header / actions */}
      <div className="flex shrink-0 flex-wrap items-center gap-2 border-b border-zinc-800 bg-panel px-3 py-2">
        <span
          className="min-w-0 max-w-[40%] truncate font-mono text-xs text-zinc-200"
          title={task.seed}
        >
          {task.seed}
        </span>
        <StatusBadge status={task.status} />
        <span className="flex items-center gap-1.5 font-mono text-2xs tabular-nums text-zinc-400">
          {running && <Spinner />}
          {task.pages} pages · {task.found} found
        </span>
        <div className="ml-auto flex items-center gap-2">
          {running ? (
            <Button variant="danger" size="sm" onClick={() => void stopTask(task.id)}>
              Stop
            </Button>
          ) : (
            <Button variant="primary" size="sm" onClick={() => void startTask(task.id)}>
              Start crawl
            </Button>
          )}
        </div>
      </div>

      {actionError && (
        <div className="shrink-0 border-b border-red-500/30 bg-red-500/10 px-3 py-1.5 text-xs text-red-400">
          {actionError}
        </div>
      )}

      {/* Discovered URLs */}
      <div className="flex min-h-0 flex-1 flex-col">
        <div className="flex shrink-0 items-center justify-between border-b border-zinc-800 bg-zinc-900/60 px-3 py-1">
          <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">
            Discovered URLs
          </span>
          {taskUrls.length > 0 && (
            <span className="font-mono text-2xs text-zinc-500">{taskUrls.length} rows</span>
          )}
        </div>
        <UrlsTable urls={taskUrls} />
      </div>
    </div>
  );
}
