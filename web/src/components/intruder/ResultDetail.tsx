import { useEffect, useMemo, useState } from 'react';
import { api, ApiError } from '../../lib/api';
import { decode, type Decoded } from '../../lib/encoding';
import { Spinner } from '../ui/Spinner';
import { statusTextClass } from '../proxy/badges';
import type { IntruderResultDetail } from '../../lib/types';

function RawPane({ title, data, empty }: { title: string; data: Decoded; empty?: boolean }) {
  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <div className="flex shrink-0 items-center justify-between border-b border-zinc-800 bg-zinc-900/60 px-3 py-1">
        <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">{title}</span>
        {!empty && (
          <span className="text-2xs text-zinc-500">{data.bytes.toLocaleString('en-US')} bytes</span>
        )}
      </div>
      <div className="min-h-0 flex-1 overflow-auto bg-canvas px-3 py-2">
        {empty ? (
          <div className="flex h-full items-center justify-center text-xs text-zinc-400">
            — no response —
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

interface ResultDetailProps {
  resultId: string;
  onClose: () => void;
}

export function ResultDetail({ resultId, onClose }: ResultDetailProps) {
  const [detail, setDetail] = useState<IntruderResultDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    setDetail(null);
    api
      .getIntruderResult(resultId)
      .then((d) => {
        if (!cancelled) {
          setDetail(d);
          setLoading(false);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof ApiError ? err.message : 'Failed to load result');
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [resultId]);

  const requestData = useMemo(
    () => (detail ? decode(detail.requestRaw) : { ok: true, text: '', bytes: 0 }),
    [detail],
  );
  const responseData = useMemo(
    () => (detail?.responseRaw ? decode(detail.responseRaw) : { ok: true, text: '', bytes: 0 }),
    [detail],
  );

  const result = detail?.result;

  return (
    <div className="flex min-h-0 flex-col border-t border-zinc-800">
      <div className="flex shrink-0 items-center gap-2 border-b border-zinc-800 bg-panel px-3 py-1.5 font-mono text-xs">
        <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">
          Result
        </span>
        {result && (
          <>
            <span className="text-zinc-500">#{result.index}</span>
            <span className="min-w-0 flex-1 truncate text-zinc-300" title={result.payloads.join(' · ')}>
              {result.payloads.join(' · ') || '—'}
            </span>
            <span className={statusTextClass(result.statusCode)}>
              {result.statusCode > 0 ? result.statusCode : '–'}
            </span>
          </>
        )}
        {!result && <span className="min-w-0 flex-1" />}
        <button
          type="button"
          onClick={onClose}
          aria-label="Close result detail"
          title="Close"
          className="shrink-0 rounded px-1 text-zinc-500 hover:text-zinc-200"
        >
          ✕
        </button>
      </div>

      {loading ? (
        <div className="flex items-center justify-center gap-2 py-8 text-xs text-zinc-500">
          <Spinner /> Loading result…
        </div>
      ) : error ? (
        <div className="flex items-center justify-center py-8 text-xs text-red-400">{error}</div>
      ) : (
        <div className="flex h-64 min-h-0 flex-col divide-y divide-zinc-800 lg:flex-row lg:divide-x lg:divide-y-0">
          <RawPane title="Request" data={requestData} />
          <RawPane
            title="Response"
            data={responseData}
            empty={!detail?.responseRaw}
          />
        </div>
      )}
    </div>
  );
}
