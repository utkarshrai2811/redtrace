import { useRef } from 'react';
import { Button } from '../ui/Button';
import { Input } from '../ui/Input';
import { Select } from '../ui/Select';
import { Toggle } from '../ui/Toggle';
import { Badge } from '../ui/Badge';
import { Spinner } from '../ui/Spinner';
import { PayloadSetEditor } from './PayloadSetEditor';
import {
  MARKER,
  MAX_CONCURRENCY,
  countPositions,
  payloadSetCount,
  projectedJobCount,
  useIntruderStore,
} from '../../store/intruderStore';
import type { AttackType, PayloadSet } from '../../lib/types';

const ATTACK_TYPES: { value: AttackType; label: string }[] = [
  { value: 'sniper', label: 'Sniper' },
  { value: 'battering_ram', label: 'Battering ram' },
  { value: 'pitchfork', label: 'Pitchfork' },
  { value: 'cluster_bomb', label: 'Cluster bomb' },
];

const STATUS_TONE = {
  pending: 'zinc',
  running: 'blue',
  completed: 'green',
  stopped: 'amber',
  error: 'red',
} as const;

export function AttackConfig() {
  const selectedId = useIntruderStore((s) => s.selectedId);
  const draft = useIntruderStore((s) => s.draft);
  const attacks = useIntruderStore((s) => s.attacks);
  const updateDraft = useIntruderStore((s) => s.updateDraft);
  const start = useIntruderStore((s) => s.start);
  const stop = useIntruderStore((s) => s.stop);
  const starting = useIntruderStore((s) => s.starting);
  const actionError = useIntruderStore((s) => s.actionError);

  const templateRef = useRef<HTMLTextAreaElement>(null);

  const attack = attacks.find((a) => a.id === selectedId);
  const running = attack?.status === 'running';

  if (!draft) return null;

  const positions = countPositions(draft.templateText);
  const needsPerPosition = draft.type === 'pitchfork' || draft.type === 'cluster_bomb';
  const editorCount = payloadSetCount(draft.type, positions);

  const markPosition = () => {
    const el = templateRef.current;
    if (!el) return;
    const { selectionStart, selectionEnd, value } = el;
    const before = value.slice(0, selectionStart);
    const selected = value.slice(selectionStart, selectionEnd);
    const after = value.slice(selectionEnd);
    const next = `${before}${MARKER}${selected}${MARKER}${after}`;
    updateDraft({ templateText: next });
    // Restore focus. With a real selection, drop the caret after the pair; with
    // an empty selection (an insertion point), place it between the markers so
    // the user can immediately type the value to fuzz.
    requestAnimationFrame(() => {
      el.focus();
      const caret = selectionStart === selectionEnd ? selectionStart + 1 : selectionEnd + 2;
      el.setSelectionRange(caret, caret);
    });
  };

  const clearPositions = () => {
    updateDraft({ templateText: draft.templateText.split(MARKER).join('') });
  };

  const updatePayloadSet = (index: number, next: PayloadSet) => {
    const payloadSets = draft.payloadSets.map((s, i) => (i === index ? next : s));
    updateDraft({ payloadSets });
  };

  // Gate Start on the projected request count, which mirrors the backend: for
  // pitchfork/cluster bomb every contributing position must have payloads, so a
  // single filled set is not enough.
  const jobCount = projectedJobCount(draft.type, positions, draft.payloadSets);
  const canStart = !running && !starting && jobCount > 0;

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-auto">
      {/* Target / config row */}
      <div className="flex shrink-0 flex-wrap items-center gap-2 border-b border-zinc-800 bg-panel px-3 py-2">
        <Input
          value={draft.name}
          placeholder="Attack name"
          aria-label="Attack name"
          onChange={(e) => updateDraft({ name: e.target.value })}
          className="w-36"
          disabled={running}
        />
        <Select
          value={draft.scheme}
          onChange={(e) => updateDraft({ scheme: e.target.value })}
          aria-label="Scheme"
          disabled={running}
        >
          <option value="http">http</option>
          <option value="https">https</option>
        </Select>
        <Input
          value={draft.host}
          placeholder="host:port"
          aria-label="Host"
          onChange={(e) => updateDraft({ host: e.target.value })}
          className="w-48 flex-1 font-mono"
          disabled={running}
        />
        <Select
          value={draft.type}
          onChange={(e) => updateDraft({ type: e.target.value as AttackType })}
          aria-label="Attack type"
          disabled={running}
        >
          {ATTACK_TYPES.map((t) => (
            <option key={t.value} value={t.value}>
              {t.label}
            </option>
          ))}
        </Select>
        <Select
          value={draft.httpVersion}
          onChange={(e) => updateDraft({ httpVersion: e.target.value })}
          aria-label="HTTP version"
          disabled={running}
        >
          <option value="HTTP/1.1">HTTP/1.1</option>
          <option value="HTTP/2">HTTP/2</option>
        </Select>
        <Toggle
          label="Follow redirects"
          checked={draft.followRedirects}
          onChange={(v) => updateDraft({ followRedirects: v })}
          disabled={running}
        />
        <label className="flex items-center gap-1.5 text-xs text-zinc-400">
          <span>Concurrency</span>
          <Input
            type="number"
            min={1}
            max={MAX_CONCURRENCY}
            value={draft.concurrency}
            aria-label="Concurrency"
            onChange={(e) =>
              updateDraft({
                concurrency: Math.min(MAX_CONCURRENCY, Math.max(1, Number(e.target.value) || 1)),
              })
            }
            className="w-16 tabular-nums"
            disabled={running}
          />
        </label>
      </div>

      {/* Actions row */}
      <div className="flex shrink-0 flex-wrap items-center gap-2 border-b border-zinc-800 bg-panel/60 px-3 py-2">
        {running ? (
          <Button variant="danger" size="sm" onClick={() => void stop()}>
            Stop
          </Button>
        ) : (
          <Button variant="primary" size="sm" disabled={!canStart} onClick={() => void start()}>
            {starting ? 'Starting…' : 'Start attack'}
          </Button>
        )}
        {attack && attack.status !== 'pending' && (
          <Badge tone={STATUS_TONE[attack.status]}>{attack.status}</Badge>
        )}
        {attack && (running || attack.total > 0) && (
          <span className="flex items-center gap-1.5 font-mono text-2xs tabular-nums text-zinc-400">
            {running && <Spinner />}
            {attack.completed} / {attack.total}
          </span>
        )}
        <span className="ml-auto font-mono text-2xs text-zinc-500">
          {jobCount.toLocaleString('en-US')} request{jobCount === 1 ? '' : 's'} · {positions}{' '}
          position{positions === 1 ? '' : 's'}
        </span>
      </div>

      {actionError && (
        <div className="shrink-0 border-b border-red-500/30 bg-red-500/10 px-3 py-1.5 text-xs text-red-400">
          {actionError}
        </div>
      )}

      {/* Request template */}
      <div className="shrink-0 border-b border-zinc-800 p-3">
        <div className="mb-1.5 flex items-center gap-2">
          <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">
            Request template
          </span>
          <Button size="sm" variant="outline" onClick={markPosition} disabled={running}>
            Mark {MARKER} position
          </Button>
          <Button size="sm" variant="ghost" onClick={clearPositions} disabled={running}>
            Clear positions
          </Button>
        </div>
        <textarea
          ref={templateRef}
          value={draft.templateText}
          spellCheck={false}
          aria-label="Request template"
          onChange={(e) => updateDraft({ templateText: e.target.value })}
          disabled={running}
          className="raw-http h-48 w-full resize-y rounded border border-zinc-700 bg-canvas px-3 py-2 text-zinc-200 focus:outline-none focus:ring-1 focus:ring-inset focus:ring-accent/40 disabled:opacity-60"
        />
      </div>

      {/* Payload sets */}
      <div className="shrink-0 space-y-2 p-3">
        {positions === 0 ? (
          <p className="text-xs text-zinc-400">
            Select text in the template and click “Mark {MARKER} position” to add a payload position.
          </p>
        ) : (
          Array.from({ length: editorCount }, (_, i) => (
            <PayloadSetEditor
              key={i}
              label={
                needsPerPosition
                  ? `Position ${i + 1}`
                  : positions > 1
                    ? `Payload set (all ${positions} positions)`
                    : 'Payload set'
              }
              set={draft.payloadSets[i] ?? { payloads: [], processors: [] }}
              onChange={(next) => updatePayloadSet(i, next)}
            />
          ))
        )}
      </div>
    </div>
  );
}
