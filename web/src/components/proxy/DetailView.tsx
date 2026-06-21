import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useProxyStore } from '../../store/proxyStore';
import { useAIStore } from '../../store/aiStore';
import { decode, type Decoded } from '../../lib/encoding';
import { api, ApiError } from '../../lib/api';
import { MethodBadge, statusTextClass } from './badges';
import { Spinner } from '../ui/Spinner';
import { Button } from '../ui/Button';

function RawPane({ title, data, empty }: { title: string; data: Decoded; empty?: boolean }) {
  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <div className="flex shrink-0 items-center justify-between border-b border-zinc-800 bg-zinc-900/60 px-3 py-1">
        <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">
          {title}
        </span>
        {!empty && (
          <span className="text-2xs text-zinc-500">{data.bytes.toLocaleString('en-US')} bytes</span>
        )}
      </div>
      <div className="min-h-0 flex-1 overflow-auto bg-canvas px-3 py-2">
        {empty ? (
          <div className="flex h-full items-center justify-center text-xs text-zinc-500">
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

export function DetailView() {
  const detail = useProxyStore((s) => s.detail);
  const loading = useProxyStore((s) => s.loadingDetail);
  const error = useProxyStore((s) => s.detailError);
  const selectedId = useProxyStore((s) => s.selectedId);
  const navigate = useNavigate();
  const [sending, setSending] = useState(false);
  const [sendError, setSendError] = useState<string | null>(null);

  const sendToRepeater = async () => {
    if (!selectedId) return;
    setSending(true);
    setSendError(null);
    try {
      await api.sendToRepeater(selectedId);
      navigate('/repeater');
    } catch (err) {
      setSendError(err instanceof ApiError ? err.message : 'Failed to send to Repeater');
      setSending(false);
    }
  };

  const sendToIntruder = async () => {
    if (!selectedId) return;
    setSending(true);
    setSendError(null);
    try {
      await api.sendToIntruder(selectedId);
      navigate('/intruder');
    } catch (err) {
      setSendError(err instanceof ApiError ? err.message : 'Failed to send to Intruder');
      setSending(false);
    }
  };

  const sendToScanner = async () => {
    if (!selectedId) return;
    setSending(true);
    setSendError(null);
    try {
      await api.sendToScanner(selectedId);
      navigate('/scanner');
    } catch (err) {
      setSendError(err instanceof ApiError ? err.message : 'Failed to send to Scanner');
      setSending(false);
    }
  };

  const sendToCrawler = async () => {
    if (!selectedId) return;
    setSending(true);
    setSendError(null);
    try {
      await api.sendToCrawler(selectedId);
      navigate('/crawler');
    } catch (err) {
      setSendError(err instanceof ApiError ? err.message : 'Failed to send to Crawler');
      setSending(false);
    }
  };

  const sendToSequencer = async () => {
    if (!selectedId) return;
    setSending(true);
    setSendError(null);
    try {
      await api.sendToSequencer(selectedId);
      navigate('/sequencer');
    } catch (err) {
      setSendError(err instanceof ApiError ? err.message : 'Failed to send to Sequencer');
      setSending(false);
    }
  };

  const requestData = useMemo(
    () => (detail ? decode(detail.requestRaw) : { ok: true, text: '', bytes: 0 }),
    [detail],
  );
  const responseData = useMemo(
    () => (detail?.responseRaw ? decode(detail.responseRaw) : { ok: true, text: '', bytes: 0 }),
    [detail],
  );

  if (!selectedId) {
    return (
      <div className="flex h-full items-center justify-center text-xs text-zinc-400">
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
        {sendError && <span className="text-2xs text-red-400">{sendError}</span>}
        <Button
          size="sm"
          variant="outline"
          disabled={sending}
          onClick={() => void sendToRepeater()}
          className="shrink-0"
        >
          {sending ? 'Sending…' : 'Send to Repeater'}
        </Button>
        <Button
          size="sm"
          variant="outline"
          disabled={sending}
          onClick={() => void sendToIntruder()}
          className="shrink-0"
        >
          Send to Intruder
        </Button>
        <Button
          size="sm"
          variant="outline"
          disabled={sending}
          onClick={() => void sendToScanner()}
          className="shrink-0"
        >
          Send to Scanner
        </Button>
        <Button
          size="sm"
          variant="outline"
          disabled={sending}
          onClick={() => void sendToCrawler()}
          className="shrink-0"
        >
          Send to Crawler
        </Button>
        <Button
          size="sm"
          variant="outline"
          disabled={sending}
          onClick={() => void sendToSequencer()}
          className="shrink-0"
        >
          Send to Sequencer
        </Button>
        <Button
          size="sm"
          variant="outline"
          onClick={() => {
            const context =
              'Explain this HTTP exchange.\n\n=== REQUEST ===\n' +
              requestData.text +
              (responseData.text ? '\n\n=== RESPONSE ===\n' + responseData.text : '');
            void useAIStore.getState().startFromContext({
              kind: 'explain',
              title: `Explain ${request.method} ${request.path}`,
              context,
            });
            navigate('/ai');
          }}
          className="shrink-0"
        >
          Send to AI
        </Button>
      </div>

      <div className="flex min-h-0 flex-1 flex-col divide-y divide-zinc-800 lg:flex-row lg:divide-x lg:divide-y-0">
        <RawPane title="Request" data={requestData} />
        <RawPane title="Response" data={responseData} empty={!hasResponse} />
      </div>
    </div>
  );
}
