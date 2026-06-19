import { useEffect, useRef, useState } from 'react';
import { PageHeader } from '../components/layout/PageHeader';
import { Button } from '../components/ui/Button';
import { Select } from '../components/ui/Select';
import { api, ApiError } from '../lib/api';
import { cn } from '../lib/cn';
import type { DecoderOp } from '../lib/types';

const OPS: { value: DecoderOp; label: string }[] = [
  { value: 'url_encode', label: 'URL encode' },
  { value: 'url_decode', label: 'URL decode' },
  { value: 'base64_encode', label: 'Base64 encode' },
  { value: 'base64_decode', label: 'Base64 decode' },
  { value: 'base64url_encode', label: 'Base64URL encode' },
  { value: 'base64url_decode', label: 'Base64URL decode' },
  { value: 'hex_encode', label: 'Hex encode' },
  { value: 'hex_decode', label: 'Hex decode' },
  { value: 'html_encode', label: 'HTML encode' },
  { value: 'html_decode', label: 'HTML decode' },
  { value: 'gzip_compress', label: 'Gzip compress' },
  { value: 'gzip_decompress', label: 'Gzip decompress' },
  { value: 'jwt_decode', label: 'JWT decode' },
];

const OP_LABELS: Record<DecoderOp, string> = Object.fromEntries(
  OPS.map((o) => [o.value, o.label]),
) as Record<DecoderOp, string>;

const DEBOUNCE_MS = 250;

interface ChainItem {
  /** Stable key so reordering keeps element identity. */
  key: string;
  op: DecoderOp;
}

export function DecoderPage() {
  const [input, setInput] = useState('');
  const [chain, setChain] = useState<ChainItem[]>([]);
  const [output, setOutput] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [pendingOp, setPendingOp] = useState<DecoderOp>('base64_decode');
  const [copied, setCopied] = useState(false);

  // Track the latest run so out-of-order responses don't clobber a newer one.
  const runToken = useRef(0);

  useEffect(() => {
    const ops = chain.map((c) => c.op);
    if (input === '' && ops.length === 0) {
      setOutput('');
      setError(null);
      return;
    }

    const token = ++runToken.current;
    const timer = window.setTimeout(() => {
      void (async () => {
        try {
          const res = await api.decoderRun(input, ops);
          if (token !== runToken.current) return;
          setOutput(res.output);
          setError(null);
        } catch (err) {
          if (token !== runToken.current) return;
          setOutput('');
          setError(err instanceof ApiError ? err.message : 'Decode failed');
        }
      })();
    }, DEBOUNCE_MS);

    return () => window.clearTimeout(timer);
  }, [input, chain]);

  const addOp = () => {
    setChain((c) => [...c, { key: crypto.randomUUID(), op: pendingOp }]);
  };

  const removeOp = (key: string) => {
    setChain((c) => c.filter((item) => item.key !== key));
  };

  const move = (index: number, delta: number) => {
    setChain((c) => {
      const next = [...c];
      const target = index + delta;
      if (target < 0 || target >= next.length) return c;
      [next[index], next[target]] = [next[target], next[index]];
      return next;
    });
  };

  const detect = async () => {
    try {
      const res = await api.decoderDetect(input);
      if (res.ok) {
        setChain((c) => [...c, { key: crypto.randomUUID(), op: res.op as DecoderOp }]);
      }
    } catch {
      // Detection is best-effort; ignore failures.
    }
  };

  const copyOutput = async () => {
    try {
      await navigator.clipboard.writeText(output);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1200);
    } catch {
      // Clipboard may be unavailable; ignore.
    }
  };

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader title="Decoder" subtitle="Transform data through a chain of encode/decode operations" />

      <div className="grid min-h-0 flex-1 grid-cols-1 gap-px bg-zinc-800 lg:grid-cols-2">
        {/* Input + chain */}
        <div className="flex min-h-0 flex-col bg-canvas">
          <div className="flex shrink-0 items-center justify-between border-b border-zinc-800 bg-zinc-900/60 px-3 py-1">
            <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">
              Input
            </span>
            <Button size="sm" variant="outline" onClick={() => void detect()} disabled={input === ''}>
              Smart detect
            </Button>
          </div>
          <textarea
            value={input}
            spellCheck={false}
            placeholder="Paste data to transform…"
            onChange={(e) => setInput(e.target.value)}
            className="raw-http min-h-0 flex-1 resize-none bg-canvas px-3 py-2 text-zinc-200 placeholder:text-zinc-600 focus:outline-none"
          />

          {/* Operation chain */}
          <div className="shrink-0 border-t border-zinc-800 bg-panel px-3 py-2">
            <div className="flex items-center gap-2 pb-2">
              <Select
                value={pendingOp}
                onChange={(e) => setPendingOp(e.target.value as DecoderOp)}
                className="flex-1"
                aria-label="Operation to add"
              >
                {OPS.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </Select>
              <Button size="sm" variant="outline" onClick={addOp}>
                + Add op
              </Button>
            </div>

            {chain.length === 0 ? (
              <p className="text-2xs text-zinc-600">
                No operations. Output mirrors the input. Add ops to build a transform chain.
              </p>
            ) : (
              <div className="space-y-1">
                {chain.map((item, i) => (
                  <div
                    key={item.key}
                    className="flex items-center gap-2 rounded-md border border-zinc-800 bg-zinc-900/40 px-2 py-1 text-xs"
                  >
                    <span className="w-5 text-right font-mono text-2xs text-zinc-600">{i + 1}.</span>
                    <span className="flex-1 text-zinc-200">{OP_LABELS[item.op]}</span>
                    <button
                      type="button"
                      title="Move up"
                      disabled={i === 0}
                      onClick={() => move(i, -1)}
                      className="rounded px-1 text-zinc-500 hover:text-zinc-200 disabled:opacity-30"
                    >
                      ↑
                    </button>
                    <button
                      type="button"
                      title="Move down"
                      disabled={i === chain.length - 1}
                      onClick={() => move(i, 1)}
                      className="rounded px-1 text-zinc-500 hover:text-zinc-200 disabled:opacity-30"
                    >
                      ↓
                    </button>
                    <button
                      type="button"
                      title="Remove operation"
                      onClick={() => removeOp(item.key)}
                      className="rounded px-1 text-zinc-500 hover:text-red-400"
                    >
                      ✕
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

        {/* Output */}
        <div className="flex min-h-0 flex-col bg-canvas">
          <div className="flex shrink-0 items-center justify-between border-b border-zinc-800 bg-zinc-900/60 px-3 py-1">
            <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">
              Output
            </span>
            <Button
              size="sm"
              variant="ghost"
              onClick={() => void copyOutput()}
              disabled={output === ''}
            >
              {copied ? 'Copied' : 'Copy'}
            </Button>
          </div>
          <div className="min-h-0 flex-1 overflow-auto px-3 py-2">
            {error ? (
              <div className="rounded-md border border-red-500/30 bg-red-500/10 px-3 py-2 text-xs text-red-400">
                {error}
              </div>
            ) : (
              <pre className={cn('raw-http', output ? 'text-zinc-200' : 'text-zinc-600')}>
                {output || '— empty —'}
              </pre>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
