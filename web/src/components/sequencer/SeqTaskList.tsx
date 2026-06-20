import { Spinner } from '../ui/Spinner';
import { cn } from '../../lib/cn';
import { StatusBadge } from './badges';
import { useSequencerStore } from '../../store/sequencerStore';

export function SeqTaskList() {
  const tasks = useSequencerStore((s) => s.tasks);
  const loading = useSequencerStore((s) => s.loadingTasks);
  const error = useSequencerStore((s) => s.tasksError);
  const selectedTaskId = useSequencerStore((s) => s.selectedTaskId);
  const selectTask = useSequencerStore((s) => s.selectTask);
  const deleteTask = useSequencerStore((s) => s.deleteTask);

  return (
    <div className="flex w-[220px] shrink-0 flex-col border-r border-zinc-800 bg-panel">
      <div className="flex shrink-0 items-center border-b border-zinc-800 px-3 py-2">
        <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">
          Captures
        </span>
      </div>

      <div className="min-h-0 flex-1 overflow-auto p-1.5">
        {loading && tasks.length === 0 ? (
          <div className="flex items-center gap-2 px-2 py-3 text-xs text-zinc-500">
            <Spinner /> Loading…
          </div>
        ) : error ? (
          <p className="px-2 py-3 text-xs text-red-400">{error}</p>
        ) : tasks.length === 0 ? (
          <p className="px-2 py-3 text-xs text-zinc-400">
            No capture tasks yet. Send a request from the proxy to analyze its tokens.
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
                        {task.collected}/{task.target}
                      </span>
                    </div>
                  </button>
                  {!running && <StatusBadge status={task.status} />}
                  <button
                    type="button"
                    title="Delete capture"
                    aria-label="Delete capture"
                    onClick={() => {
                      const msg = running
                        ? 'This capture is running and will be stopped. Delete it?'
                        : 'Delete this capture?';
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
