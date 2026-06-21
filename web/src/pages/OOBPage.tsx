import { useEffect, useState } from 'react';
import { PageHeader } from '../components/layout/PageHeader';
import { Button } from '../components/ui/Button';
import { Spinner } from '../components/ui/Spinner';
import { ConfigNotice } from '../components/oob/ConfigNotice';
import { PayloadsList } from '../components/oob/PayloadsList';
import { InteractionsTable } from '../components/oob/InteractionsTable';
import { InteractionDetail } from '../components/oob/InteractionDetail';
import { useOOBStore } from '../store/oobStore';
import type { OOBConfig } from '../lib/types';

function ConfigStrip({ config }: { config: OOBConfig }) {
  const clearInteractions = useOOBStore((s) => s.clearInteractions);
  const count = useOOBStore((s) => s.interactions.length);

  return (
    <div className="flex shrink-0 flex-wrap items-center gap-x-4 gap-y-1 border-b border-zinc-800 bg-panel/40 px-3 py-2">
      <div className="flex items-baseline gap-1.5">
        <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">Domain</span>
        <span className="font-mono text-xs text-zinc-200">{config.domain}</span>
      </div>
      <div className="flex items-baseline gap-1.5">
        <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">IP</span>
        <span className="font-mono text-xs text-zinc-300">{config.publicIp}</span>
      </div>
      <div className="flex items-baseline gap-1.5">
        <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">HTTP</span>
        <span className="font-mono text-xs text-zinc-400">{config.httpAddr}</span>
      </div>
      <div className="flex items-baseline gap-1.5">
        <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">DNS</span>
        <span className="font-mono text-xs text-zinc-400">{config.dnsAddr}</span>
      </div>
      <div className="ml-auto flex items-center gap-2">
        <span className="text-2xs text-zinc-500">
          {count.toLocaleString('en-US')} interaction{count === 1 ? '' : 's'}
        </span>
        <Button
          size="sm"
          variant="ghost"
          onClick={() => {
            if (window.confirm('Clear all interactions? This cannot be undone.')) {
              void clearInteractions();
            }
          }}
          title="Clear all interactions"
        >
          <svg
            width="13"
            height="13"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden="true"
          >
            <path d="M3 6h18M8 6V4h8v2M6 6l1 14h10l1-14" />
          </svg>
          Clear
        </Button>
      </div>
    </div>
  );
}

function CollaboratorView({ config }: { config: OOBConfig }) {
  const interactions = useOOBStore((s) => s.interactions);
  const [selectedId, setSelectedId] = useState<string | null>(null);

  // Drop the selection if the interaction was cleared from the table.
  useEffect(() => {
    if (selectedId && !interactions.some((i) => i.id === selectedId)) {
      setSelectedId(null);
    }
  }, [interactions, selectedId]);

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <ConfigStrip config={config} />
      <div className="flex min-h-0 flex-1">
        <PayloadsList />
        <div className="flex min-h-0 min-w-0 flex-1 flex-col">
          <div className="flex shrink-0 items-center justify-between border-b border-zinc-800 bg-zinc-900/60 px-3 py-1">
            <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">
              Interactions
            </span>
          </div>
          <InteractionsTable
            interactions={interactions}
            selectedId={selectedId}
            onSelect={setSelectedId}
          />
          {selectedId && (
            <InteractionDetail interactionId={selectedId} onClose={() => setSelectedId(null)} />
          )}
        </div>
      </div>
    </div>
  );
}

export function OOBPage() {
  const config = useOOBStore((s) => s.config);
  const loading = useOOBStore((s) => s.loading);
  const error = useOOBStore((s) => s.error);
  const fetchAll = useOOBStore((s) => s.fetchAll);

  // The backend is the source of truth; reload on every mount so payloads and
  // interactions persist across navigation and live WS frames fold in on top.
  useEffect(() => {
    void fetchAll();
  }, [fetchAll]);

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader title="Collaborator" subtitle="Out-of-band interaction capture (DNS / HTTP)" />
      {error ? (
        <div className="flex min-h-0 flex-1 items-center justify-center text-xs text-red-400">
          {error}
        </div>
      ) : loading && !config ? (
        <div className="flex min-h-0 flex-1 items-center justify-center gap-2 text-xs text-zinc-500">
          <Spinner /> Loading collaborator…
        </div>
      ) : config && !config.enabled ? (
        <ConfigNotice />
      ) : config ? (
        <CollaboratorView config={config} />
      ) : null}
    </div>
  );
}
