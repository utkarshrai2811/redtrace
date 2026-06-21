import { cn } from '../../lib/cn';
import { formatTime } from '../../lib/format';
import { KindBadge } from './KindBadge';
import { useAIStore } from '../../store/aiStore';

export function ConversationList() {
  const conversations = useAIStore((s) => s.conversations);
  const activeId = useAIStore((s) => s.activeId);
  const openConversation = useAIStore((s) => s.openConversation);
  const deleteConversation = useAIStore((s) => s.deleteConversation);
  const clearConversations = useAIStore((s) => s.clearConversations);

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="min-h-0 flex-1 overflow-auto">
        {conversations.length === 0 ? (
          <div className="px-3 py-6 text-center text-2xs text-zinc-500">No conversations yet.</div>
        ) : (
          <ul className="space-y-0.5 p-1.5">
            {conversations.map((c) => (
              <li key={c.id}>
                <div
                  className={cn(
                    'group flex items-center gap-2 rounded-md px-2 py-1.5 transition-colors',
                    c.id === activeId ? 'bg-accent/10' : 'hover:bg-zinc-800/40',
                  )}
                >
                  <button
                    type="button"
                    onClick={() => void openConversation(c.id)}
                    className="flex min-w-0 flex-1 flex-col items-start gap-1 text-left focus-visible:outline-none"
                  >
                    <div className="flex w-full items-center gap-1.5">
                      <KindBadge kind={c.kind} />
                      <span className="min-w-0 flex-1 truncate text-xs text-zinc-200" title={c.title}>
                        {c.title || 'Untitled'}
                      </span>
                    </div>
                    <span className="font-mono text-2xs tabular-nums text-zinc-500">
                      {formatTime(c.updatedAt)}
                    </span>
                  </button>
                  <button
                    type="button"
                    onClick={() => {
                      if (window.confirm('Delete this conversation?')) {
                        void deleteConversation(c.id);
                      }
                    }}
                    aria-label="Delete conversation"
                    title="Delete conversation"
                    className="shrink-0 rounded px-1 text-zinc-600 opacity-0 transition-opacity hover:text-zinc-200 focus-visible:opacity-100 group-hover:opacity-100"
                  >
                    ✕
                  </button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
      {conversations.length > 0 && (
        <div className="shrink-0 border-t border-zinc-800 p-1.5">
          <button
            type="button"
            onClick={() => {
              if (window.confirm('Clear all conversations? This cannot be undone.')) {
                void clearConversations();
              }
            }}
            className="w-full rounded-md px-2 py-1 text-left text-2xs text-zinc-500 hover:bg-zinc-800/40 hover:text-zinc-300 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent/60"
          >
            Clear all
          </button>
        </div>
      )}
    </div>
  );
}
