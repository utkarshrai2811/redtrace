import { useEffect, useMemo, useState } from 'react';
import { PageHeader } from '../components/layout/PageHeader';
import { IssuesToolbar } from '../components/scanner/IssuesToolbar';
import { IssuesTable } from '../components/scanner/IssuesTable';
import { IssueDetail } from '../components/scanner/IssueDetail';
import { ScanList } from '../components/scanner/ScanList';
import { ScanDetail } from '../components/scanner/ScanDetail';
import { cn } from '../lib/cn';
import { useScannerStore } from '../store/scannerStore';

type Tab = 'issues' | 'scans';

function TabButton({
  active,
  count,
  label,
  onClick,
}: {
  active: boolean;
  count?: number;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={cn(
        'flex items-center gap-1.5 border-b-2 px-3 py-2 text-xs font-medium transition-colors',
        'focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent/60',
        active
          ? 'border-accent text-zinc-100'
          : 'border-transparent text-zinc-400 hover:text-zinc-200',
      )}
    >
      {label}
      {count !== undefined && count > 0 && (
        <span className="rounded bg-zinc-800 px-1 py-0.5 font-mono text-[9px] tabular-nums text-zinc-400">
          {count}
        </span>
      )}
    </button>
  );
}

function IssuesView() {
  const issues = useScannerStore((s) => s.issues);
  const loading = useScannerStore((s) => s.loadingIssues);
  const error = useScannerStore((s) => s.issuesError);
  const severityFilter = useScannerStore((s) => s.severityFilter);
  const hostFilter = useScannerStore((s) => s.hostFilter);

  const [selectedIssueId, setSelectedIssueId] = useState<string | null>(null);

  // Apply the severity chips and host substring filter to the (already sorted)
  // issue list. An empty severity set means "all severities".
  const filtered = useMemo(() => {
    const host = hostFilter.trim().toLowerCase();
    return issues.filter((i) => {
      if (severityFilter.size > 0 && !severityFilter.has(i.severity)) return false;
      if (host && !i.host.toLowerCase().includes(host)) return false;
      return true;
    });
  }, [issues, severityFilter, hostFilter]);

  // Drop the selection if the issue no longer matches the filter / was cleared.
  useEffect(() => {
    if (selectedIssueId && !filtered.some((i) => i.id === selectedIssueId)) {
      setSelectedIssueId(null);
    }
  }, [filtered, selectedIssueId]);

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <IssuesToolbar count={filtered.length} />
      {error ? (
        <div className="flex flex-1 items-center justify-center text-xs text-red-400">{error}</div>
      ) : loading && issues.length === 0 ? (
        <div className="flex flex-1 items-center justify-center text-xs text-zinc-500">
          Loading findings…
        </div>
      ) : (
        <>
          <IssuesTable
            issues={filtered}
            selectedIssueId={selectedIssueId}
            onSelect={setSelectedIssueId}
          />
          {selectedIssueId && (
            <IssueDetail issueId={selectedIssueId} onClose={() => setSelectedIssueId(null)} />
          )}
        </>
      )}
    </div>
  );
}

function ScansView() {
  return (
    <div className="flex min-h-0 flex-1">
      <div className="flex w-[220px] shrink-0 flex-col border-r border-zinc-800 bg-panel">
        <div className="flex shrink-0 items-center border-b border-zinc-800 px-3 py-2">
          <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">
            Scans
          </span>
        </div>
        <ScanList />
      </div>
      <ScanDetail />
    </div>
  );
}

export function ScannerPage() {
  const [tab, setTab] = useState<Tab>('issues');
  const fetchIssues = useScannerStore((s) => s.fetchIssues);
  const fetchTasks = useScannerStore((s) => s.fetchTasks);
  const issueCount = useScannerStore((s) => s.issues.length);
  const taskCount = useScannerStore((s) => s.tasks.length);

  // The backend is the source of truth; reload both on mount so findings and
  // scans created elsewhere (e.g. "Send to Scanner") appear.
  useEffect(() => {
    void fetchIssues();
    void fetchTasks();
  }, [fetchIssues, fetchTasks]);

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader title="Scanner" subtitle="Passive findings and active vulnerability scans" />
      <div className="flex shrink-0 items-center gap-1 border-b border-zinc-800 bg-panel/60 px-2">
        <TabButton
          active={tab === 'issues'}
          count={issueCount}
          label="Issues"
          onClick={() => setTab('issues')}
        />
        <TabButton
          active={tab === 'scans'}
          count={taskCount}
          label="Scans"
          onClick={() => setTab('scans')}
        />
      </div>
      {tab === 'issues' ? <IssuesView /> : <ScansView />}
    </div>
  );
}
