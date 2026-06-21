import { useEffect } from 'react';
import { Link } from 'react-router-dom';
import { PageHeader } from '../components/layout/PageHeader';
import { Button } from '../components/ui/Button';
import { Spinner } from '../components/ui/Spinner';
import { ConversationList } from '../components/ai/ConversationList';
import { MessageThread } from '../components/ai/MessageThread';
import { Composer } from '../components/ai/Composer';
import { AIDisabledNotice } from '../components/ai/AIDisabledNotice';
import { useAIStore } from '../store/aiStore';

function DisabledBanner() {
  return (
    <div className="flex shrink-0 items-center gap-2 border-b border-amber-500/30 bg-amber-500/10 px-3 py-1.5 text-2xs text-amber-400">
      <span>AI is disabled — replies will not generate until a provider is configured.</span>
      <Link to="/settings" className="underline hover:text-amber-300">
        Open Settings
      </Link>
    </div>
  );
}

export function AIPage() {
  const config = useAIStore((s) => s.config);
  const loading = useAIStore((s) => s.loading);
  const error = useAIStore((s) => s.error);
  const activeId = useAIStore((s) => s.activeId);
  const fetchConfig = useAIStore((s) => s.fetchConfig);
  const fetchConversations = useAIStore((s) => s.fetchConversations);
  const newChat = useAIStore((s) => s.newChat);

  // The backend is the source of truth; reload config + conversations on mount.
  useEffect(() => {
    void fetchConfig();
    void fetchConversations();
  }, [fetchConfig, fetchConversations]);

  const disabled = Boolean(config && !config.enabled);

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader title="AI" subtitle="AI-assisted analysis, triage & payloads" />

      <div className="flex min-h-0 flex-1">
        {/* Conversations */}
        <div className="flex w-[240px] shrink-0 flex-col border-r border-zinc-800">
          <div className="shrink-0 border-b border-zinc-800 p-2">
            <Button variant="primary" size="sm" className="w-full" onClick={() => void newChat()}>
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
                <path d="M12 5v14M5 12h14" />
              </svg>
              New chat
            </Button>
          </div>
          <ConversationList />
        </div>

        {/* Active conversation */}
        <div className="flex min-h-0 min-w-0 flex-1 flex-col">
          {error ? (
            <div className="flex min-h-0 flex-1 items-center justify-center text-xs text-red-400">
              {error}
            </div>
          ) : loading && !config ? (
            <div className="flex min-h-0 flex-1 items-center justify-center gap-2 text-xs text-zinc-500">
              <Spinner /> Loading AI…
            </div>
          ) : activeId ? (
            <>
              {disabled && <DisabledBanner />}
              <MessageThread />
              <Composer />
            </>
          ) : disabled ? (
            <AIDisabledNotice />
          ) : (
            <div className="flex min-h-0 flex-1 items-center justify-center text-xs text-zinc-500">
              Select or start a conversation
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
