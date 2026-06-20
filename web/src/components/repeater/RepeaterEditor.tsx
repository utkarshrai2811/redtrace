import { useMemo } from 'react';
import { Button } from '../ui/Button';
import { Input } from '../ui/Input';
import { Select } from '../ui/Select';
import { Toggle } from '../ui/Toggle';
import { Spinner } from '../ui/Spinner';
import { ResponsePane } from './ResponsePane';
import { decode } from '../../lib/encoding';
import { formatDuration, formatTime } from '../../lib/format';
import { cn } from '../../lib/cn';
import { statusTextClass } from '../proxy/badges';
import { useRepeaterStore } from '../../store/repeaterStore';

export function RepeaterEditor() {
  const selectedId = useRepeaterStore((s) => s.selectedId);
  const draft = useRepeaterStore((s) => s.draft);
  const loading = useRepeaterStore((s) => s.loadingDetail);
  const detailError = useRepeaterStore((s) => s.detailError);
  const sending = useRepeaterStore((s) => s.sending);
  const sendError = useRepeaterStore((s) => s.sendError);
  const history = useRepeaterStore((s) => s.history);
  const selectedHistoryIndex = useRepeaterStore((s) => s.selectedHistoryIndex);
  const updateDraft = useRepeaterStore((s) => s.updateDraft);
  const selectHistory = useRepeaterStore((s) => s.selectHistory);
  const saveAndSend = useRepeaterStore((s) => s.saveAndSend);

  const current = selectedHistoryIndex >= 0 ? history[selectedHistoryIndex] : undefined;

  // When viewing a past history entry, show that entry's recorded request
  // (read-only) instead of the editable draft.
  const viewingPast = Boolean(current);
  const pastRequest = useMemo(
    () => (current ? decode(current.requestRaw) : null),
    [current],
  );

  const responseRaw = current?.responseRaw ?? '';
  const responseStatus = current?.statusCode ?? 0;
  const responseDuration = current?.durationMs ?? 0;

  if (!selectedId) {
    return (
      <div className="flex h-full flex-1 items-center justify-center text-xs text-zinc-400">
        Select or create a tab to start.
      </div>
    );
  }

  if (loading) {
    return (
      <div className="flex h-full flex-1 items-center justify-center gap-2 text-xs text-zinc-500">
        <Spinner /> Loading tab…
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

  if (!draft) return null;

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      {/* Control row */}
      <div className="flex shrink-0 flex-wrap items-center gap-2 border-b border-zinc-800 bg-panel px-3 py-2">
        <Input
          value={draft.name}
          placeholder="Tab name"
          onChange={(e) => updateDraft({ name: e.target.value })}
          className="w-32"
        />
        <Select
          value={draft.scheme}
          onChange={(e) => updateDraft({ scheme: e.target.value })}
          aria-label="Scheme"
        >
          <option value="http">http</option>
          <option value="https">https</option>
        </Select>
        <Input
          value={draft.host}
          placeholder="host:port"
          onChange={(e) => updateDraft({ host: e.target.value })}
          className="w-48 flex-1 font-mono"
        />
        <Select
          value={draft.httpVersion}
          onChange={(e) => updateDraft({ httpVersion: e.target.value })}
          aria-label="HTTP version"
        >
          <option value="HTTP/1.1">HTTP/1.1</option>
          <option value="HTTP/2">HTTP/2</option>
        </Select>
        <Toggle
          label="Follow redirects"
          checked={draft.followRedirects}
          onChange={(v) => updateDraft({ followRedirects: v })}
        />
        <Button
          variant="primary"
          size="sm"
          className="ml-auto"
          disabled={sending}
          onClick={() => void saveAndSend()}
        >
          {sending ? 'Sending…' : 'Send'}
        </Button>
      </div>

      {sendError && (
        <div className="shrink-0 border-b border-red-500/30 bg-red-500/10 px-3 py-1.5 text-xs text-red-400">
          {sendError}
        </div>
      )}

      {/* Request / Response panes */}
      <div className="flex min-h-0 flex-1 flex-col divide-y divide-zinc-800 lg:flex-row lg:divide-x lg:divide-y-0">
        {/* Request */}
        <div className="flex min-h-0 min-w-0 flex-1 flex-col">
          <div className="flex shrink-0 items-center justify-between border-b border-zinc-800 bg-zinc-900/60 px-3 py-1">
            <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">
              Request
            </span>
            {viewingPast && (
              <button
                type="button"
                onClick={() => selectHistory(-1)}
                className="text-2xs text-accent-fg hover:text-accent"
              >
                ← back to editor
              </button>
            )}
          </div>
          {viewingPast && pastRequest ? (
            <div className="min-h-0 flex-1 overflow-auto bg-canvas px-3 py-2">
              <pre className="raw-http text-zinc-300">{pastRequest.text}</pre>
            </div>
          ) : (
            <textarea
              value={draft.raw}
              spellCheck={false}
              onChange={(e) => updateDraft({ raw: e.target.value })}
              className="raw-http min-h-0 flex-1 resize-none bg-canvas px-3 py-2 text-zinc-200 focus:outline-none"
            />
          )}
        </div>

        {/* Response */}
        <ResponsePane
          responseRaw={responseRaw}
          statusCode={responseStatus}
          durationMs={responseDuration}
          sending={sending}
        />
      </div>

      {/* History strip */}
      {history.length > 0 && (
        <div className="flex shrink-0 items-center gap-2 border-t border-zinc-800 bg-panel px-3 py-1.5">
          <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">
            History
          </span>
          <div className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto">
            {history.map((h, i) => (
              <button
                key={h.id}
                type="button"
                onClick={() => selectHistory(i)}
                title={`${formatTime(h.createdAt)} · ${formatDuration(h.durationMs)}`}
                className={cn(
                  'flex shrink-0 items-center gap-1.5 rounded border px-1.5 py-0.5 font-mono text-2xs tabular-nums transition-colors',
                  i === selectedHistoryIndex
                    ? 'border-accent/40 bg-accent/15'
                    : 'border-zinc-700 bg-zinc-900/40 hover:bg-zinc-800',
                )}
              >
                <span className="text-zinc-500">#{i + 1}</span>
                <span className={statusTextClass(h.statusCode)}>
                  {h.statusCode > 0 ? h.statusCode : '–'}
                </span>
                <span className="text-zinc-500">{formatDuration(h.durationMs)}</span>
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
