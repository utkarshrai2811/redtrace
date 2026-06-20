import { useEffect, useState } from 'react';
import { PageHeader } from '../components/layout/PageHeader';
import { AttackList } from '../components/intruder/AttackList';
import { AttackConfig } from '../components/intruder/AttackConfig';
import { ResultsTable } from '../components/intruder/ResultsTable';
import { ResultDetail } from '../components/intruder/ResultDetail';
import { Spinner } from '../components/ui/Spinner';
import { useIntruderStore } from '../store/intruderStore';

function MainPanel() {
  const selectedId = useIntruderStore((s) => s.selectedId);
  const draft = useIntruderStore((s) => s.draft);
  const loading = useIntruderStore((s) => s.loadingDetail);
  const detailError = useIntruderStore((s) => s.detailError);
  const results = useIntruderStore((s) => s.results);

  const [selectedResultId, setSelectedResultId] = useState<string | null>(null);

  // Clear any open result detail when switching attacks.
  useEffect(() => {
    setSelectedResultId(null);
  }, [selectedId]);

  // Drop the selection if the result no longer exists (e.g. after a restart).
  useEffect(() => {
    if (selectedResultId && !results.some((r) => r.id === selectedResultId)) {
      setSelectedResultId(null);
    }
  }, [results, selectedResultId]);

  if (!selectedId) {
    return (
      <div className="flex h-full flex-1 items-center justify-center text-xs text-zinc-400">
        Select or create an attack to start.
      </div>
    );
  }

  if (loading) {
    return (
      <div className="flex h-full flex-1 items-center justify-center gap-2 text-xs text-zinc-500">
        <Spinner /> Loading attack…
      </div>
    );
  }

  if (detailError) {
    return (
      <div className="flex h-full flex-1 items-center justify-center text-xs text-red-400">
        {detailError}
      </div>
    );
  }

  if (!draft) return null;

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <AttackConfig />

      {/* Results */}
      <div className="flex min-h-0 flex-1 flex-col border-t border-zinc-800">
        <div className="flex shrink-0 items-center justify-between border-b border-zinc-800 bg-zinc-900/60 px-3 py-1">
          <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">
            Results
          </span>
          {results.length > 0 && (
            <span className="font-mono text-2xs text-zinc-500">{results.length} rows</span>
          )}
        </div>
        <ResultsTable
          results={results}
          selectedResultId={selectedResultId}
          onSelect={setSelectedResultId}
        />
        {selectedResultId && (
          <ResultDetail resultId={selectedResultId} onClose={() => setSelectedResultId(null)} />
        )}
      </div>
    </div>
  );
}

export function IntruderPage() {
  const fetchAttacks = useIntruderStore((s) => s.fetchAttacks);

  // The backend is the source of truth; reload on every mount so attacks
  // created elsewhere (e.g. "Send to Intruder") appear.
  useEffect(() => {
    void fetchAttacks();
  }, [fetchAttacks]);

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader title="Intruder" subtitle="Automate customized attacks with payload positions" />
      <div className="flex min-h-0 flex-1">
        <AttackList />
        <MainPanel />
      </div>
    </div>
  );
}
