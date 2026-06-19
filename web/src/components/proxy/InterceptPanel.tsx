import { useEffect, useState } from 'react';
import { useProxyStore } from '../../store/proxyStore';
import { decodeBase64, encodeBase64 } from '../../lib/encoding';
import { Button } from '../ui/Button';
import { Badge } from '../ui/Badge';
import type { HeldItem } from '../../lib/types';

function HeldEditor({ item }: { item: HeldItem }) {
  const forwardItem = useProxyStore((s) => s.forwardItem);
  const dropItem = useProxyStore((s) => s.dropItem);

  const original = decodeBase64(item.raw);
  const [text, setText] = useState(original);
  const [busy, setBusy] = useState(false);

  // Reset the editor whenever a different item reaches the head of the queue.
  useEffect(() => {
    setText(decodeBase64(item.raw));
  }, [item.id, item.raw]);

  const edited = text !== original;

  const handleForward = async () => {
    setBusy(true);
    try {
      await forwardItem(item.id, edited ? encodeBase64(text) : undefined);
    } finally {
      setBusy(false);
    }
  };

  const handleDrop = async () => {
    setBusy(true);
    try {
      await dropItem(item.id);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex shrink-0 items-center gap-2 border-b border-zinc-800 bg-zinc-900/60 px-3 py-1.5 font-mono text-xs">
        <Badge tone={item.direction === 'request' ? 'amber' : 'blue'}>
          {item.direction === 'request' ? 'REQUEST' : 'RESPONSE'}
        </Badge>
        {item.method && <span className="text-zinc-400">{item.method}</span>}
        <span className="min-w-0 flex-1 truncate text-zinc-300" title={item.url ?? item.host}>
          {item.url ?? item.host ?? ''}
        </span>
        {typeof item.statusCode === 'number' && item.statusCode > 0 && (
          <span className="text-zinc-400">{item.statusCode}</span>
        )}
        {edited && <Badge tone="accent">edited</Badge>}
      </div>

      <textarea
        value={text}
        onChange={(e) => setText(e.target.value)}
        spellCheck={false}
        className="raw-http min-h-0 flex-1 resize-none bg-canvas px-3 py-2 text-zinc-200 outline-none"
      />

      <div className="flex shrink-0 items-center justify-end gap-2 border-t border-zinc-800 bg-panel px-3 py-2">
        <Button variant="danger" size="sm" disabled={busy} onClick={() => void handleDrop()}>
          Drop
        </Button>
        <Button variant="primary" size="sm" disabled={busy} onClick={() => void handleForward()}>
          Forward{edited ? ' (edited)' : ''}
        </Button>
      </div>
    </div>
  );
}

export function InterceptPanel() {
  const intercept = useProxyStore((s) => s.intercept);
  const head = intercept.queue[0];

  if (intercept.count === 0 || !head) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-6">
      <div className="flex h-[80vh] w-full max-w-3xl flex-col overflow-hidden rounded-lg border border-accent/40 bg-panel shadow-2xl shadow-black/50">
        <div className="flex shrink-0 items-center justify-between border-b border-zinc-800 px-4 py-2.5">
          <div className="flex items-center gap-2">
            <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-accent" />
            <h2 className="text-sm font-semibold text-zinc-100">Intercept</h2>
            <Badge tone="accent">{intercept.count} held</Badge>
          </div>
          <span className="text-2xs text-zinc-500">
            Editing head item — forward or drop to continue
          </span>
        </div>
        <HeldEditor key={head.id} item={head} />
      </div>
    </div>
  );
}
