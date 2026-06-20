import { useEffect, useMemo, useState } from 'react';
import { api, ApiError } from '../../lib/api';
import { decode, type Decoded } from '../../lib/encoding';
import { Spinner } from '../ui/Spinner';
import { SeverityBadge, ConfidenceBadge, OriginBadge } from './badges';
import type { ScanIssueDetail } from '../../lib/types';

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

function Field({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="space-y-0.5">
      <div className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">{label}</div>
      <div className={mono ? 'whitespace-pre-wrap break-words font-mono text-xs text-zinc-300' : 'text-xs text-zinc-300'}>
        {value || '—'}
      </div>
    </div>
  );
}

interface IssueDetailProps {
  issueId: string;
  onClose: () => void;
}

export function IssueDetail({ issueId, onClose }: IssueDetailProps) {
  const [detail, setDetail] = useState<ScanIssueDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    setDetail(null);
    api
      .getScanIssue(issueId)
      .then((d) => {
        if (!cancelled) {
          setDetail(d);
          setLoading(false);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof ApiError ? err.message : 'Failed to load issue');
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [issueId]);

  const requestData = useMemo(
    () => (detail ? decode(detail.requestRaw) : { ok: true, text: '', bytes: 0 }),
    [detail],
  );
  const responseData = useMemo(
    () => (detail?.responseRaw ? decode(detail.responseRaw) : { ok: true, text: '', bytes: 0 }),
    [detail],
  );

  const issue = detail?.issue;

  return (
    <div className="flex min-h-0 flex-col border-t border-zinc-800">
      <div className="flex shrink-0 items-center gap-2 border-b border-zinc-800 bg-panel px-3 py-1.5 text-xs">
        <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">Issue</span>
        {issue && (
          <>
            <SeverityBadge severity={issue.severity} />
            <span className="min-w-0 flex-1 truncate text-zinc-200" title={issue.name}>
              {issue.name}
            </span>
            <ConfidenceBadge confidence={issue.confidence} />
            <OriginBadge origin={issue.origin} />
          </>
        )}
        {!issue && <span className="min-w-0 flex-1" />}
        <button
          type="button"
          onClick={onClose}
          aria-label="Close issue detail"
          title="Close"
          className="shrink-0 rounded px-1 text-zinc-500 hover:text-zinc-200 focus-visible:opacity-100"
        >
          ✕
        </button>
      </div>

      {loading ? (
        <div className="flex items-center justify-center gap-2 py-8 text-xs text-zinc-500">
          <Spinner /> Loading issue…
        </div>
      ) : error ? (
        <div className="flex items-center justify-center py-8 text-xs text-red-400">{error}</div>
      ) : issue ? (
        <div className="flex min-h-0 flex-1 flex-col divide-y divide-zinc-800 lg:flex-row lg:divide-x lg:divide-y-0">
          {/* Metadata */}
          <div className="min-h-0 w-full shrink-0 space-y-3 overflow-auto bg-panel/40 px-3 py-3 lg:w-[340px]">
            <Field
              label="Location"
              value={`${issue.method} ${issue.scheme}://${issue.host}${issue.path}`}
              mono
            />
            {issue.param && <Field label="Parameter" value={issue.param} mono />}
            {issue.payload && <Field label="Payload" value={issue.payload} mono />}
            <Field label="Detail" value={issue.detail} />
            <Field label="Evidence" value={issue.evidence} mono />
            <Field label="Remediation" value={issue.remediation} />
          </div>
          {/* Raw request / response */}
          <div className="flex h-64 min-h-0 flex-1 flex-col divide-y divide-zinc-800 lg:h-auto lg:flex-row lg:divide-x lg:divide-y-0">
            <RawPane title="Request" data={requestData} empty={requestData.bytes === 0} />
            <RawPane title="Response" data={responseData} empty={!detail?.responseRaw} />
          </div>
        </div>
      ) : null}
    </div>
  );
}
