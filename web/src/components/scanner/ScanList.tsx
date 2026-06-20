import { cn } from '../../lib/cn';
import { Spinner } from '../ui/Spinner';
import { StatusBadge } from './badges';
import { useScannerStore } from '../../store/scannerStore';

export function ScanList() {
  const tasks = useScannerStore((s) => s.tasks);
  const loading = useScannerStore((s) => s.loadingTasks);
  const error = useScannerStore((s) => s.tasksError);
  const selectedTaskId = useScannerStore((s) => s.selectedTaskId);
  const selectTask = useScannerStore((s) => s.selectTask);
  const deleteTask = useScannerStore((s) => s.deleteTask);

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="min-h-0 flex-1 overflow-auto p-1.5">
        {loading && tasks.length === 0 ? (
          <div className="flex items-center gap-2 px-2 py-3 text-xs text-zinc-500">
            <Spinner /> Loading…
          </div>
        ) : error ? (
          <p className="px-2 py-3 text-xs text-red-400">{error}</p>
        ) : tasks.length === 0 ? (
          <p className="px-2 py-3 text-xs text-zinc-400">
            No scans yet. Send a request from the proxy to scan it.
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
                    title={task.host ? `${task.scheme}://${task.host}` : task.name}
                  >
                    <div className="truncate">{task.name || 'Untitled'}</div>
                    <div className="flex items-center gap-1.5">
                      <span className="truncate font-mono text-2xs text-zinc-500">
                        {task.host || 'no host'}
                      </span>
                      <span className="flex shrink-0 items-center gap-1 font-mono text-2xs tabular-nums text-zinc-500">
                        {running && <Spinner className="h-2.5 w-2.5" />}
                        {task.completed}/{task.total}
                      </span>
                      {task.issues > 0 && (
                        <span className="shrink-0 font-mono text-2xs text-amber-400">
                          {task.issues} issue{task.issues === 1 ? '' : 's'}
                        </span>
                      )}
                    </div>
                  </button>
                  {!running && <StatusBadge status={task.status} />}
                  <button
                    type="button"
                    title="Delete scan"
                    aria-label="Delete scan"
                    onClick={() => {
                      const msg = running
                        ? 'This scan is running and will be stopped. Delete it?'
                        : 'Delete this scan?';
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
