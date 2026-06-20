import { useEffect, useMemo, useRef, useState } from 'react';
import { PageHeader } from '../components/layout/PageHeader';
import { Select } from '../components/ui/Select';
import { api, ApiError } from '../lib/api';
import { cn } from '../lib/cn';
import type { ComparerMode, DiffSegment } from '../lib/types';

const DEBOUNCE_MS = 300;

const SEGMENT_CLASS: Record<DiffSegment['op'], string> = {
  equal: 'text-zinc-400',
  insert: 'bg-emerald-500/20 text-emerald-300',
  delete: 'bg-red-500/20 text-red-300 line-through',
};

export function ComparerPage() {
  const [a, setA] = useState('');
  const [b, setB] = useState('');
  const [mode, setMode] = useState<ComparerMode>('lines');
  const [segments, setSegments] = useState<DiffSegment[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const runToken = useRef(0);

  useEffect(() => {
    if (a === '' && b === '') {
      setSegments(null);
      setError(null);
      return;
    }

    const token = ++runToken.current;
    const timer = window.setTimeout(() => {
      void (async () => {
        try {
          const res = await api.comparer(a, b, mode);
          if (token !== runToken.current) return;
          setSegments(res.segments);
          setError(null);
        } catch (err) {
          if (token !== runToken.current) return;
          setSegments(null);
          setError(err instanceof ApiError ? err.message : 'Compare failed');
        }
      })();
    }, DEBOUNCE_MS);

    return () => window.clearTimeout(timer);
  }, [a, b, mode]);

  const counts = useMemo(() => {
    if (!segments) return { inserted: 0, deleted: 0 };
    let inserted = 0;
    let deleted = 0;
    for (const seg of segments) {
      if (seg.op === 'insert') inserted += 1;
      else if (seg.op === 'delete') deleted += 1;
    }
    return { inserted, deleted };
  }, [segments]);

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader
        title="Comparer"
        subtitle="Diff two payloads by line or word"
        actions={
          <div className="flex items-center gap-2">
            <Select
              value={mode}
              onChange={(e) => setMode(e.target.value as ComparerMode)}
              aria-label="Diff mode"
            >
              <option value="lines">Lines</option>
              <option value="words">Words</option>
            </Select>
          </div>
        }
      />

      {/* Inputs */}
      <div className="grid shrink-0 grid-cols-2 gap-px bg-zinc-800" style={{ height: '38%' }}>
        {(
          [
            { label: 'A', value: a, set: setA },
            { label: 'B', value: b, set: setB },
          ] as const
        ).map((side) => (
          <div key={side.label} className="flex min-h-0 flex-col bg-canvas">
            <div className="shrink-0 border-b border-zinc-800 bg-zinc-900/60 px-3 py-1 text-2xs font-semibold uppercase tracking-wider text-zinc-500">
              {side.label}
            </div>
            <textarea
              value={side.value}
              spellCheck={false}
              placeholder={`Paste payload ${side.label}…`}
              onChange={(e) => side.set(e.target.value)}
              className="raw-http min-h-0 flex-1 resize-none bg-canvas px-3 py-2 text-zinc-200 placeholder:text-zinc-600 focus:outline-none"
            />
          </div>
        ))}
      </div>

      {/* Diff result */}
      <div className="flex min-h-0 flex-1 flex-col border-t border-zinc-800">
        <div className="flex shrink-0 items-center gap-3 border-b border-zinc-800 bg-zinc-900/60 px-3 py-1">
          <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">Diff</span>
          {segments && (
            <div className="flex items-center gap-2 font-mono text-2xs tabular-nums">
              <span className="text-emerald-400">+{counts.inserted}</span>
              <span className="text-red-400">-{counts.deleted}</span>
            </div>
          )}
        </div>
        <div className="min-h-0 flex-1 overflow-auto px-3 py-2">
          {error ? (
            <div className="rounded-md border border-red-500/30 bg-red-500/10 px-3 py-2 text-xs text-red-400">
              {error}
            </div>
          ) : !segments ? (
            <div className="flex h-full items-center justify-center text-xs text-zinc-400">
              Enter text in A and B to compare.
            </div>
          ) : (
            <pre className="raw-http whitespace-pre-wrap">
              {segments.map((seg, i) => (
                <span key={i} className={cn(SEGMENT_CLASS[seg.op])}>
                  {seg.text}
                </span>
              ))}
            </pre>
          )}
        </div>
      </div>
    </div>
  );
}
