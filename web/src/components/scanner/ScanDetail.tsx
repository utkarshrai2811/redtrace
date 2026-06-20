import { useEffect, useMemo, useState } from 'react';
import { decode } from '../../lib/encoding';
import { Button } from '../ui/Button';
import { Spinner } from '../ui/Spinner';
import { StatusBadge } from './badges';
import { IssuesTable } from './IssuesTable';
import { IssueDetail } from './IssueDetail';
import { useScannerStore } from '../../store/scannerStore';

export function ScanDetail() {
  const selectedTaskId = useScannerStore((s) => s.selectedTaskId);
  const task = useScannerStore((s) => s.selectedTask);
  const taskIssues = useScannerStore((s) => s.taskIssues);
  const loading = useScannerStore((s) => s.loadingDetail);
  const detailError = useScannerStore((s) => s.detailError);
  const actionError = useScannerStore((s) => s.actionError);
  const startTask = useScannerStore((s) => s.startTask);
  const stopTask = useScannerStore((s) => s.stopTask);

  const [selectedIssueId, setSelectedIssueId] = useState<string | null>(null);

  // Clear any open issue detail when switching scans.
  useEffect(() => {
    setSelectedIssueId(null);
  }, [selectedTaskId]);

  // Drop the selection if the issue no longer exists.
  useEffect(() => {
    if (selectedIssueId && !taskIssues.some((i) => i.id === selectedIssueId)) {
      setSelectedIssueId(null);
    }
  }, [taskIssues, selectedIssueId]);

  const template = useMemo(
    () => (task ? decode(task.template) : { ok: true, text: '', bytes: 0 }),
    [task],
  );

  if (!selectedTaskId) {
    return (
      <div className="flex h-full flex-1 items-center justify-center text-xs text-zinc-400">
        Select a scan to view its configuration and findings.
      </div>
    );
  }

  if (loading) {
    return (
      <div className="flex h-full flex-1 items-center justify-center gap-2 text-xs text-zinc-500">
        <Spinner /> Loading scan…
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
        <span className="min-w-0 truncate text-sm font-semibold text-zinc-100" title={task.name}>
          {task.name || 'Untitled'}
        </span>
        <span className="truncate font-mono text-2xs text-zinc-400">
          {task.scheme}://{task.host}
        </span>
        <StatusBadge status={task.status} />
        <span className="flex items-center gap-1.5 font-mono text-2xs tabular-nums text-zinc-400">
          {running && <Spinner />}
          {task.completed} / {task.total}
        </span>
        {task.issues > 0 && (
          <span className="font-mono text-2xs text-amber-400">
            {task.issues} issue{task.issues === 1 ? '' : 's'}
          </span>
        )}
        <div className="ml-auto flex items-center gap-2">
          {running ? (
            <Button variant="danger" size="sm" onClick={() => void stopTask(task.id)}>
              Stop
            </Button>
          ) : (
            <Button variant="primary" size="sm" onClick={() => void startTask(task.id)}>
              Start scan
            </Button>
          )}
        </div>
      </div>

      {actionError && (
        <div className="shrink-0 border-b border-red-500/30 bg-red-500/10 px-3 py-1.5 text-xs text-red-400">
          {actionError}
        </div>
      )}

      {/* Request template (read-only) */}
      <div className="shrink-0 border-b border-zinc-800 p-3">
        <div className="mb-1.5 text-2xs font-semibold uppercase tracking-wider text-zinc-500">
          Request template
        </div>
        <pre className="raw-http max-h-40 overflow-auto rounded border border-zinc-700 bg-canvas px-3 py-2 text-zinc-300">
          {template.ok ? template.text : 'could not decode (binary content)'}
        </pre>
      </div>

      {/* Task findings */}
      <div className="flex min-h-0 flex-1 flex-col">
        <div className="flex shrink-0 items-center justify-between border-b border-zinc-800 bg-zinc-900/60 px-3 py-1">
          <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">
            Findings
          </span>
          {taskIssues.length > 0 && (
            <span className="font-mono text-2xs text-zinc-500">{taskIssues.length} rows</span>
          )}
        </div>
        <IssuesTable
          issues={taskIssues}
          selectedIssueId={selectedIssueId}
          onSelect={setSelectedIssueId}
        />
        {selectedIssueId && (
          <IssueDetail issueId={selectedIssueId} onClose={() => setSelectedIssueId(null)} />
        )}
      </div>
    </div>
  );
}
