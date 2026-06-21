import { Badge } from '../ui/Badge';
import { CopyButton } from './CopyButton';
import { PayloadGenerator } from './PayloadGenerator';
import { useOOBStore } from '../../store/oobStore';

export function PayloadsList() {
  const payloads = useOOBStore((s) => s.payloads);

  return (
    <div className="flex w-[280px] shrink-0 flex-col border-r border-zinc-800 bg-panel">
      <div className="flex shrink-0 items-center justify-between border-b border-zinc-800 px-3 py-2">
        <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">
          Payloads
        </span>
        {payloads.length > 0 && (
          <span className="font-mono text-2xs text-zinc-500">{payloads.length}</span>
        )}
      </div>

      <PayloadGenerator />

      <div className="min-h-0 flex-1 overflow-auto p-1.5">
        {payloads.length === 0 ? (
          <p className="px-2 py-3 text-xs text-zinc-400">
            No payloads yet. Generate one above, then plant its host or URL where you suspect
            out-of-band interaction.
          </p>
        ) : (
          <div className="space-y-0.5">
            {payloads.map((p) => (
              <div
                key={p.token}
                className="group flex items-center gap-1.5 rounded-md px-2 py-1.5 text-sm text-zinc-300"
              >
                <span className="min-w-0 flex-1 truncate font-mono text-xs" title={p.host}>
                  {p.host}
                </span>
                <CopyButton
                  value={p.host}
                  label="Copy payload host"
                  className="opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100 focus-visible:opacity-100"
                />
                <Badge
                  tone={p.interactions > 0 ? 'green' : 'zinc'}
                  className="shrink-0 tabular-nums"
                  title={`${p.interactions} interaction${p.interactions === 1 ? '' : 's'}`}
                >
                  {p.interactions}
                </Badge>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
