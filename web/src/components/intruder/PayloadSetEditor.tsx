import { useState } from 'react';
import { Button } from '../ui/Button';
import { Input } from '../ui/Input';
import { Select } from '../ui/Select';
import type { PayloadSet, Processor, ProcessorKind } from '../../lib/types';

const PROCESSOR_KINDS: { value: ProcessorKind; label: string }[] = [
  { value: 'prefix', label: 'Add prefix' },
  { value: 'suffix', label: 'Add suffix' },
  { value: 'base64', label: 'Base64-encode' },
  { value: 'base64url', label: 'Base64url-encode' },
  { value: 'url', label: 'URL-encode' },
  { value: 'upper', label: 'Uppercase' },
  { value: 'lower', label: 'Lowercase' },
  { value: 'sha256', label: 'SHA-256 hash' },
  { value: 'md5', label: 'MD5 hash' },
];

const KIND_LABELS: Record<ProcessorKind, string> = PROCESSOR_KINDS.reduce(
  (acc, k) => {
    acc[k.value] = k.label;
    return acc;
  },
  {} as Record<ProcessorKind, string>,
);

/** Kinds that take a free-text value (everything else is a pure transform). */
function takesValue(kind: ProcessorKind): boolean {
  return kind === 'prefix' || kind === 'suffix';
}

interface PayloadSetEditorProps {
  label: string;
  set: PayloadSet;
  onChange: (next: PayloadSet) => void;
}

export function PayloadSetEditor({ label, set, onChange }: PayloadSetEditorProps) {
  const [newKind, setNewKind] = useState<ProcessorKind>('prefix');
  const [newValue, setNewValue] = useState('');

  const payloadsText = set.payloads.join('\n');
  const processors = set.processors ?? [];

  const setPayloads = (text: string) => {
    // Store the raw newline-split lines so the textarea round-trips faithfully
    // (including blank lines being typed); empty lines are filtered for counts
    // and on save (see the store's input mapping).
    onChange({ ...set, payloads: text.split('\n'), processors });
  };

  const addProcessor = () => {
    const proc: Processor = takesValue(newKind)
      ? { kind: newKind, value: newValue }
      : { kind: newKind };
    onChange({ ...set, processors: [...processors, proc] });
    setNewValue('');
  };

  const removeProcessor = (index: number) => {
    onChange({ ...set, processors: processors.filter((_, i) => i !== index) });
  };

  const payloadCount = set.payloads.filter((p) => p.length > 0).length;

  return (
    <div className="rounded-md border border-zinc-800 bg-zinc-900/40">
      <div className="flex items-center justify-between border-b border-zinc-800 px-3 py-1.5">
        <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">
          {label}
        </span>
        <span className="font-mono text-2xs text-zinc-500">{payloadCount} payloads</span>
      </div>

      <div className="p-2">
        <textarea
          value={payloadsText}
          spellCheck={false}
          aria-label={`${label} payloads`}
          placeholder="One payload per line…"
          onChange={(e) => setPayloads(e.target.value)}
          className="raw-http h-28 w-full resize-y rounded border border-zinc-700 bg-canvas px-2 py-1.5 text-zinc-200 placeholder:text-zinc-500 focus:outline-none focus:ring-1 focus:ring-inset focus:ring-accent/40"
        />

        {/* Processor chain */}
        <div className="mt-2">
          <span className="text-2xs font-medium uppercase tracking-wider text-zinc-500">
            Processors
          </span>
          {processors.length > 0 && (
            <div className="mt-1 space-y-1">
              {processors.map((proc, i) => (
                <div
                  key={i}
                  className="flex items-center gap-2 rounded border border-zinc-800 bg-zinc-900/60 px-2 py-1 text-xs"
                >
                  <span className="font-mono text-zinc-500">{i + 1}.</span>
                  <span className="flex-1 text-zinc-300">
                    {KIND_LABELS[proc.kind]}
                    {takesValue(proc.kind) && proc.value ? (
                      <span className="ml-1 font-mono text-zinc-500">“{proc.value}”</span>
                    ) : null}
                  </span>
                  <button
                    type="button"
                    aria-label={`Remove processor ${i + 1}`}
                    title="Remove processor"
                    onClick={() => removeProcessor(i)}
                    className="rounded px-1 text-zinc-600 hover:text-red-400"
                  >
                    ✕
                  </button>
                </div>
              ))}
            </div>
          )}

          <div className="mt-1 flex items-center gap-2">
            <Select
              value={newKind}
              aria-label="Processor kind"
              onChange={(e) => setNewKind(e.target.value as ProcessorKind)}
              className="flex-1"
            >
              {PROCESSOR_KINDS.map((k) => (
                <option key={k.value} value={k.value}>
                  {k.label}
                </option>
              ))}
            </Select>
            {takesValue(newKind) && (
              <Input
                value={newValue}
                placeholder="value"
                aria-label="Processor value"
                onChange={(e) => setNewValue(e.target.value)}
                className="w-28"
              />
            )}
            <Button size="sm" variant="outline" onClick={addProcessor}>
              + Add
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}
