import { useMemo } from 'react';
import { useProxyStore } from '../../store/proxyStore';
import { decodeBase64 } from '../../lib/encoding';
import { MethodBadge, statusTextClass } from './badges';
import { Spinner } from '../ui/Spinner';

function RawPane({ title, content, empty }: { title: string; content: string; empty?: boolean }) {
  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <div className="flex shrink-0 items-center justify-between border-b border-zinc-800 bg-zinc-900/60 px-3 py-1">
        <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">
          {title}
        </span>
        {!empty && <span className="text-2xs text-zinc-600">{content.length} bytes</span>}
      </div>
      <div className="min-h-0 flex-1 overflow-auto bg-canvas px-3 py-2">
        {empty ? (
          <div className="flex h-full items-center justify-center text-xs text-zinc-600">
            — no response —
          </div>
        ) : (
          <pre className="raw-http text-zinc-300">{content}</pre>
        )}
      </div>
    </div>
  );
}

export function DetailView() {
  const detail = useProxyStore((s) => s.detail);
  const loading = useProxyStore((s) => s.loadingDetail);
  const error = useProxyStore((s) => s.detailError);
  const selectedId = useProxyStore((s) => s.selectedId);

  const requestText = useMemo(
    () => (detail ? decodeBase64(detail.requestRaw) : ''),
    [detail],
  );
  const responseText = useMemo(
    () => (detail?.responseRaw ? decodeBase64(detail.responseRaw) : ''),
    [detail],
  );

  if (!selectedId) {
    return (
      <div className="flex h-full items-center justify-center text-xs text-zinc-600">
        Select a request to inspect it
      </div>
    );
  }

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center gap-2 text-xs text-zinc-500">
        <Spinner /> Loading request…
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex h-full items-center justify-center text-xs text-red-400">{error}</div>
    );
  }

  if (!detail) return null;

  const { request, response } = detail;
  const hasResponse = Boolean(detail.responseRaw);

  return (
    <div className="flex h-full min-h-0 flex-col">
      {/* Summary line */}
      <div className="flex shrink-0 items-center gap-2 border-b border-zinc-800 bg-panel px-3 py-1.5 font-mono text-xs">
        <MethodBadge method={request.method} />
        <span className="min-w-0 flex-1 truncate text-zinc-300" title={request.url}>
          {request.url}
        </span>
        {response && (
          <span className={statusTextClass(response.statusCode)}>
            {response.statusCode}
            {response.reason ? ` ${response.reason}` : ''}
          </span>
        )}
      </div>

      <div className="flex min-h-0 flex-1 flex-col divide-y divide-zinc-800 lg:flex-row lg:divide-x lg:divide-y-0">
        <RawPane title="Request" content={requestText} />
        <RawPane title="Response" content={responseText} empty={!hasResponse} />
      </div>
    </div>
  );
}
