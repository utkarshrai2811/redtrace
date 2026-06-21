import { useState } from 'react';
import { Button } from '../ui/Button';
import { Spinner } from '../ui/Spinner';
import { CopyButton } from './CopyButton';
import { useOOBStore } from '../../store/oobStore';
import type { OOBPayloadView } from '../../lib/types';

/** A single labelled URL/host line with a copy affordance. */
function PayloadLine({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center gap-2">
      <span className="w-10 shrink-0 text-2xs font-semibold uppercase tracking-wider text-zinc-500">
        {label}
      </span>
      <span className="min-w-0 flex-1 truncate font-mono text-xs text-zinc-200" title={value}>
        {value}
      </span>
      <CopyButton value={value} label={`Copy ${label} payload`} />
    </div>
  );
}

export function PayloadGenerator() {
  const generating = useOOBStore((s) => s.generating);
  const actionError = useOOBStore((s) => s.actionError);
  const generatePayload = useOOBStore((s) => s.generatePayload);

  // The most recently generated payload, surfaced prominently so it can be
  // copied into a test target. Cleared on each new generation until it returns.
  const [latest, setLatest] = useState<OOBPayloadView | null>(null);

  const generate = async () => {
    const payload = await generatePayload();
    if (payload) setLatest(payload);
  };

  return (
    <div className="shrink-0 border-b border-zinc-800 bg-panel/40 px-3 py-3">
      <div className="flex items-center gap-2">
        <Button
          size="sm"
          variant="primary"
          disabled={generating}
          onClick={() => void generate()}
        >
          {generating && <Spinner className="h-3 w-3 border-white/40 border-t-white" />}
          Generate payload
        </Button>
        <span className="text-2xs text-zinc-500">
          Mints a unique host that captures DNS/HTTP callbacks.
        </span>
      </div>

      {actionError && <p className="mt-2 text-xs text-red-400">{actionError}</p>}

      {latest && (
        <div className="mt-3 space-y-1.5 rounded-md border border-accent/30 bg-accent/5 px-3 py-2.5">
          <div className="mb-1 text-2xs font-semibold uppercase tracking-wider text-accent-fg">
            New payload
          </div>
          <PayloadLine label="Host" value={latest.host} />
          <PayloadLine label="HTTP" value={latest.httpUrl} />
        </div>
      )}
    </div>
  );
}
