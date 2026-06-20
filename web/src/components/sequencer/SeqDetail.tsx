import { useMemo } from 'react';
import { decode } from '../../lib/encoding';
import { Button } from '../ui/Button';
import { Input } from '../ui/Input';
import { Select } from '../ui/Select';
import { Spinner } from '../ui/Spinner';
import { StatusBadge } from './badges';
import { AnalysisPanel } from './AnalysisPanel';
import { useSequencerStore } from '../../store/sequencerStore';
import type { SeqSource } from '../../lib/types';

export function SeqDetail() {
  const selectedTaskId = useSequencerStore((s) => s.selectedTaskId);
  const task = useSequencerStore((s) => s.selectedTask);
  const draft = useSequencerStore((s) => s.draft);
  const report = useSequencerStore((s) => s.report);
  const tokens = useSequencerStore((s) => s.tokens);
  const loading = useSequencerStore((s) => s.loadingDetail);
  const detailError = useSequencerStore((s) => s.detailError);
  const actionError = useSequencerStore((s) => s.actionError);
  const updateDraft = useSequencerStore((s) => s.updateDraft);
  const saveDraft = useSequencerStore((s) => s.saveDraft);
  const startTask = useSequencerStore((s) => s.startTask);
  const stopTask = useSequencerStore((s) => s.stopTask);

  const template = useMemo(
    () => (task ? decode(task.template) : { ok: true, text: '', bytes: 0 }),
    [task],
  );

  if (!selectedTaskId) {
    return (
      <div className="flex h-full flex-1 items-center justify-center text-xs text-zinc-400">
        Select a capture task to configure and analyze its tokens.
      </div>
    );
  }

  if (loading) {
    return (
      <div className="flex h-full flex-1 items-center justify-center gap-2 text-xs text-zinc-500">
        <Spinner /> Loading task…
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

  if (!task || !draft) return null;

  const running = task.status === 'running';
  const selectorLabel = draft.source === 'cookie' ? 'Cookie name' : 'Regex';

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-auto">
      {/* Header */}
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
          {task.collected} / {task.target}
        </span>
      </div>

      {/* Config row */}
      <div className="flex shrink-0 flex-wrap items-center gap-2 border-b border-zinc-800 bg-panel/60 px-3 py-2">
        <label className="flex items-center gap-1.5 text-xs text-zinc-400">
          <span>Source</span>
          <Select
            value={draft.source}
            aria-label="Token source"
            disabled={running}
            onChange={(e) => updateDraft({ source: e.target.value as SeqSource })}
          >
            <option value="cookie">Cookie</option>
            <option value="regex">Regex</option>
          </Select>
        </label>
        <label className="flex flex-1 items-center gap-1.5 text-xs text-zinc-400">
          <span className="shrink-0">{selectorLabel}</span>
          <Input
            value={draft.selector}
            placeholder={draft.source === 'cookie' ? 'session' : '([A-Za-z0-9]+)'}
            aria-label={selectorLabel}
            disabled={running}
            onChange={(e) => updateDraft({ selector: e.target.value })}
            className="w-full min-w-[8rem] font-mono"
          />
        </label>
        <label className="flex items-center gap-1.5 text-xs text-zinc-400">
          <span>Target</span>
          <Input
            type="number"
            min={1}
            value={draft.target}
            aria-label="Target sample count"
            disabled={running}
            onChange={(e) => updateDraft({ target: Math.max(1, Number(e.target.value) || 1) })}
            className="w-20 tabular-nums"
          />
        </label>
        <div className="ml-auto flex items-center gap-2">
          <Button size="sm" variant="outline" disabled={running} onClick={() => void saveDraft()}>
            Save
          </Button>
          {running ? (
            <Button variant="danger" size="sm" onClick={() => void stopTask(task.id)}>
              Stop
            </Button>
          ) : (
            <Button variant="primary" size="sm" onClick={() => void startTask(task.id)}>
              Start capture
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

      {/* Analysis */}
      {report ? (
        <AnalysisPanel report={report} tokens={tokens} />
      ) : (
        <div className="flex flex-1 items-center justify-center py-10 text-xs text-zinc-400">
          No analysis yet. Start the capture to collect tokens and generate a report.
        </div>
      )}
    </div>
  );
}
