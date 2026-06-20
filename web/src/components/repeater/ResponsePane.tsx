import { useMemo } from 'react';
import { decode } from '../../lib/encoding';
import { formatDuration } from '../../lib/format';
import { statusTextClass } from '../proxy/badges';
import { Spinner } from '../ui/Spinner';

interface ResponsePaneProps {
  /** Base64 response raw, or empty when there is no response yet. */
  responseRaw: string;
  statusCode: number;
  durationMs: number;
  sending: boolean;
}

export function ResponsePane({ responseRaw, statusCode, durationMs, sending }: ResponsePaneProps) {
  const data = useMemo(() => decode(responseRaw), [responseRaw]);

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <div className="flex shrink-0 items-center justify-between border-b border-zinc-800 bg-zinc-900/60 px-3 py-1">
        <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">
          Response
        </span>
        <div className="flex items-center gap-3 font-mono text-2xs tabular-nums">
          {statusCode > 0 && (
            <span className={statusTextClass(statusCode)}>{statusCode}</span>
          )}
          {durationMs > 0 && <span className="text-zinc-500">{formatDuration(durationMs)}</span>}
          {responseRaw && (
            <span className="text-zinc-500">{data.bytes.toLocaleString('en-US')} bytes</span>
          )}
        </div>
      </div>
      <div className="min-h-0 flex-1 overflow-auto bg-canvas px-3 py-2">
        {sending ? (
          <div className="flex h-full items-center justify-center gap-2 text-xs text-zinc-500">
            <Spinner /> Sending…
          </div>
        ) : !responseRaw ? (
          <div className="flex h-full items-center justify-center text-xs text-zinc-400">
            — no response yet —
          </div>
        ) : !data.ok ? (
          <div className="flex h-full items-center justify-center text-xs text-zinc-500">
            could not decode (binary content)
          </div>
        ) : (
          <pre className="raw-http text-zinc-300">{data.text}</pre>
        )}
      </div>
    </div>
  );
}
