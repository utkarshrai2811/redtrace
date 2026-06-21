import { useEffect, useMemo, useState } from 'react';
import { api, ApiError } from '../../lib/api';
import { decode, type Decoded } from '../../lib/encoding';
import { formatTime } from '../../lib/format';
import { Spinner } from '../ui/Spinner';
import { ProtocolBadge } from './badges';
import type { OOBInteractionDetail } from '../../lib/types';

function Field({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="space-y-0.5">
      <div className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">{label}</div>
      <div
        className={
          mono
            ? 'whitespace-pre-wrap break-words font-mono text-xs text-zinc-300'
            : 'text-xs text-zinc-300'
        }
      >
        {value || '—'}
      </div>
    </div>
  );
}

interface InteractionDetailProps {
  interactionId: string;
  onClose: () => void;
}

export function InteractionDetail({ interactionId, onClose }: InteractionDetailProps) {
  const [detail, setDetail] = useState<OOBInteractionDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    setDetail(null);
    api
      .oobInteraction(interactionId)
      .then((d) => {
        if (!cancelled) {
          setDetail(d);
          setLoading(false);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof ApiError ? err.message : 'Failed to load interaction');
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [interactionId]);

  // The raw payload is base64 of attacker-controlled callback bytes; decode for
  // display only and render strictly as text (never as HTML).
  const raw: Decoded = useMemo(
    () => (detail ? decode(detail.raw) : { ok: true, text: '', bytes: 0 }),
    [detail],
  );

  const interaction = detail?.interaction;

  return (
    <div className="flex min-h-0 flex-col border-t border-zinc-800">
      <div className="flex shrink-0 items-center gap-2 border-b border-zinc-800 bg-panel px-3 py-1.5 text-xs">
        <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">
          Interaction
        </span>
        {interaction && (
          <>
            <ProtocolBadge protocol={interaction.protocol} />
            <span className="min-w-0 flex-1 truncate font-mono text-zinc-200" title={interaction.sourceIp}>
              {interaction.sourceIp}
            </span>
            <span className="shrink-0 font-mono text-2xs tabular-nums text-zinc-500">
              {formatTime(interaction.createdAt)}
            </span>
          </>
        )}
        {!interaction && <span className="min-w-0 flex-1" />}
        <button
          type="button"
          onClick={onClose}
          aria-label="Close interaction detail"
          title="Close"
          className="shrink-0 rounded px-1 text-zinc-500 hover:text-zinc-200 focus-visible:opacity-100"
        >
          ✕
        </button>
      </div>

      {loading ? (
        <div className="flex items-center justify-center gap-2 py-8 text-xs text-zinc-500">
          <Spinner /> Loading interaction…
        </div>
      ) : error ? (
        <div className="flex items-center justify-center py-8 text-xs text-red-400">{error}</div>
      ) : interaction ? (
        <div className="flex min-h-0 flex-1 flex-col divide-y divide-zinc-800 lg:flex-row lg:divide-x lg:divide-y-0">
          {/* Metadata */}
          <div className="min-h-0 w-full shrink-0 space-y-3 overflow-auto bg-panel/40 px-3 py-3 lg:w-[340px]">
            <Field label="Protocol" value={interaction.protocol} mono />
            <Field label="Source IP" value={interaction.sourceIp} mono />
            <Field label="Token" value={interaction.token ?? ''} mono />
            <Field label="Query" value={interaction.query} mono />
            <Field label="Detail" value={interaction.detail} />
          </div>
          {/* Raw callback bytes */}
          <div className="flex h-64 min-h-0 flex-1 flex-col lg:h-auto">
            <div className="flex shrink-0 items-center justify-between border-b border-zinc-800 bg-zinc-900/60 px-3 py-1">
              <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">
                Raw
              </span>
              {raw.ok && (
                <span className="text-2xs text-zinc-500">
                  {raw.bytes.toLocaleString('en-US')} bytes
                </span>
              )}
            </div>
            <div className="min-h-0 flex-1 overflow-auto bg-canvas px-3 py-2">
              {raw.bytes === 0 ? (
                <div className="flex h-full items-center justify-center text-xs text-zinc-500">
                  — no raw payload —
                </div>
              ) : !raw.ok ? (
                <div className="flex h-full items-center justify-center text-xs text-zinc-500">
                  could not decode (binary content)
                </div>
              ) : (
                <pre className="raw-http text-zinc-300">{raw.text}</pre>
              )}
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}
