import { Button } from '../ui/Button';
import { Spinner } from '../ui/Spinner';
import { cn } from '../../lib/cn';
import { useIntruderStore } from '../../store/intruderStore';

export function AttackList() {
  const attacks = useIntruderStore((s) => s.attacks);
  const loading = useIntruderStore((s) => s.loadingAttacks);
  const error = useIntruderStore((s) => s.attacksError);
  const selectedId = useIntruderStore((s) => s.selectedId);
  const createAttack = useIntruderStore((s) => s.createAttack);
  const selectAttack = useIntruderStore((s) => s.selectAttack);
  const deleteAttack = useIntruderStore((s) => s.deleteAttack);

  return (
    <div className="flex w-[200px] shrink-0 flex-col border-r border-zinc-800 bg-panel">
      <div className="flex shrink-0 items-center justify-between border-b border-zinc-800 px-3 py-2">
        <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">Attacks</span>
        <Button size="sm" variant="outline" onClick={() => void createAttack()}>
          + New
        </Button>
      </div>

      <div className="min-h-0 flex-1 overflow-auto p-1.5">
        {loading && attacks.length === 0 ? (
          <div className="flex items-center gap-2 px-2 py-3 text-xs text-zinc-500">
            <Spinner /> Loading…
          </div>
        ) : error ? (
          <p className="px-2 py-3 text-xs text-red-400">{error}</p>
        ) : attacks.length === 0 ? (
          <p className="px-2 py-3 text-xs text-zinc-400">
            No attacks yet. Create one or send a request from the proxy.
          </p>
        ) : (
          <div className="space-y-0.5">
            {attacks.map((attack) => {
              const running = attack.status === 'running';
              return (
                <div
                  key={attack.id}
                  className={cn(
                    'group flex items-center gap-1.5 rounded-md px-2 py-1.5 text-sm transition-colors',
                    attack.id === selectedId
                      ? 'bg-zinc-800/80 text-zinc-100'
                      : 'text-zinc-400 hover:bg-zinc-800/50 hover:text-zinc-200',
                  )}
                >
                  <button
                    type="button"
                    onClick={() => void selectAttack(attack.id)}
                    className="min-w-0 flex-1 truncate text-left"
                    title={attack.host ? `${attack.scheme}://${attack.host}` : attack.name}
                  >
                    {attack.name || 'Untitled'}
                    <span className="ml-1 truncate font-mono text-2xs text-zinc-500">
                      {running ? `${attack.completed}/${attack.total}` : attack.status}
                    </span>
                  </button>
                  <button
                    type="button"
                    title="Delete attack"
                    aria-label="Delete attack"
                    onClick={() => void deleteAttack(attack.id)}
                    className="shrink-0 rounded px-1 text-zinc-600 opacity-0 transition-opacity hover:text-red-400 group-hover:opacity-100"
                  >
                    ✕
                  </button>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
