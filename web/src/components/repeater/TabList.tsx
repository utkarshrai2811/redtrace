import { Button } from '../ui/Button';
import { Spinner } from '../ui/Spinner';
import { cn } from '../../lib/cn';
import { useRepeaterStore } from '../../store/repeaterStore';

export function TabList() {
  const tabs = useRepeaterStore((s) => s.tabs);
  const loading = useRepeaterStore((s) => s.loadingTabs);
  const error = useRepeaterStore((s) => s.tabsError);
  const selectedId = useRepeaterStore((s) => s.selectedId);
  const createTab = useRepeaterStore((s) => s.createTab);
  const selectTab = useRepeaterStore((s) => s.selectTab);
  const deleteTab = useRepeaterStore((s) => s.deleteTab);

  return (
    <div className="flex w-[200px] shrink-0 flex-col border-r border-zinc-800 bg-panel">
      <div className="flex shrink-0 items-center justify-between border-b border-zinc-800 px-3 py-2">
        <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">Tabs</span>
        <Button size="sm" variant="outline" onClick={() => void createTab()}>
          + New
        </Button>
      </div>

      <div className="min-h-0 flex-1 overflow-auto p-1.5">
        {loading && tabs.length === 0 ? (
          <div className="flex items-center gap-2 px-2 py-3 text-xs text-zinc-500">
            <Spinner /> Loading…
          </div>
        ) : error ? (
          <p className="px-2 py-3 text-xs text-red-400">{error}</p>
        ) : tabs.length === 0 ? (
          <p className="px-2 py-3 text-xs text-zinc-600">
            No tabs yet. Create one or send a request from the proxy.
          </p>
        ) : (
          <div className="space-y-0.5">
            {tabs.map((tab) => (
              <div
                key={tab.id}
                className={cn(
                  'group flex items-center gap-1.5 rounded-md px-2 py-1.5 text-sm transition-colors',
                  tab.id === selectedId
                    ? 'bg-zinc-800/80 text-zinc-100'
                    : 'text-zinc-400 hover:bg-zinc-800/50 hover:text-zinc-200',
                )}
              >
                <button
                  type="button"
                  onClick={() => void selectTab(tab.id)}
                  className="min-w-0 flex-1 truncate text-left"
                  title={tab.host ? `${tab.scheme}://${tab.host}` : tab.name}
                >
                  {tab.name || 'Untitled'}
                  {tab.host && (
                    <span className="ml-1 truncate font-mono text-2xs text-zinc-500">
                      {tab.host}
                    </span>
                  )}
                </button>
                <button
                  type="button"
                  title="Delete tab"
                  onClick={() => void deleteTab(tab.id)}
                  className="shrink-0 rounded px-1 text-zinc-600 opacity-0 transition-opacity hover:text-red-400 group-hover:opacity-100"
                >
                  ✕
                </button>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
