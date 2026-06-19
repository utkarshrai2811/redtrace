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
  const forwardAll = useProxyStore((s) => s.forwardAll);
  const dropAll = useProxyStore((s) => s.dropAll);
  const setInterceptEnabled = useProxyStore((s) => s.setInterceptEnabled);
  const head = intercept.queue[0];

  if (!head) return null;

  // A bottom-docked panel (not a full-screen modal): the toolbar and the top of
  // the traffic table stay visible and usable, and there are always escape
  // hatches so the operator is never trapped behind a flooding queue.
  return (
    <div className="absolute inset-x-0 bottom-0 z-40 flex h-[55%] flex-col border-t-2 border-accent/60 bg-panel shadow-2xl shadow-black/60">
      <div className="flex shrink-0 flex-wrap items-center gap-2 border-b border-zinc-800 bg-zinc-900/70 px-3 py-2">
        <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-accent" />
        <h2 className="text-sm font-semibold text-zinc-100">Intercept</h2>
        <Badge tone="accent">{intercept.count} held</Badge>
        <span className="hidden text-2xs text-zinc-500 sm:inline">
          in-scope traffic only — set a scope to focus
        </span>
        <div className="ml-auto flex items-center gap-2">
          <Button size="sm" variant="ghost" onClick={() => void forwardAll()}>
            Forward all
          </Button>
          <Button size="sm" variant="danger" onClick={() => void dropAll()}>
            Drop all
          </Button>
          <Button
            size="sm"
            variant="ghost"
            title="Turn interception off (forwards everything currently held)"
            onClick={() => void setInterceptEnabled(false)}
          >
            Stop intercept
          </Button>
        </div>
      </div>
      <HeldEditor key={head.id} item={head} />
    </div>
  );
}
