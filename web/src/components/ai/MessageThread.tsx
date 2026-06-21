import { useEffect, useRef } from 'react';
import { Link } from 'react-router-dom';
import { Spinner } from '../ui/Spinner';
import { Markdownish } from './Markdownish';
import { useAIStore } from '../../store/aiStore';

export function MessageThread() {
  const messages = useAIStore((s) => s.messages);
  const streaming = useAIStore((s) => s.streaming);
  const draft = useAIStore((s) => s.draft);
  const streamError = useAIStore((s) => s.streamError);

  const bottomRef = useRef<HTMLDivElement>(null);

  // Keep the latest content in view as messages arrive and the draft grows.
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ block: 'end' });
  }, [messages.length, draft, streamError, streaming]);

  return (
    <div className="min-h-0 flex-1 overflow-auto px-4 py-4">
      <div className="mx-auto flex max-w-3xl flex-col gap-3">
        {messages.map((m) =>
          m.role === 'user' ? (
            <div key={m.id} className="flex justify-end">
              <div className="max-w-[85%] whitespace-pre-wrap break-words rounded-lg border border-zinc-700 bg-zinc-800/70 px-3 py-2 text-xs text-zinc-100">
                {m.content}
              </div>
            </div>
          ) : (
            <div key={m.id} className="flex justify-start">
              <div className="max-w-[90%] rounded-lg border border-zinc-800 bg-panel px-3 py-2 text-xs leading-relaxed">
                <Markdownish text={m.content} />
              </div>
            </div>
          ),
        )}

        {streaming && (
          <div className="flex justify-start">
            <div className="max-w-[90%] rounded-lg border border-zinc-800 bg-panel px-3 py-2 text-xs leading-relaxed">
              {draft ? (
                <Markdownish text={draft} />
              ) : (
                <span className="flex items-center gap-2 text-zinc-500">
                  <Spinner /> Thinking…
                </span>
              )}
            </div>
          </div>
        )}

        {streamError && (
          <div className="rounded-md border border-red-500/30 bg-red-500/10 px-3 py-2 text-2xs text-red-400">
            {streamError}{' '}
            <Link to="/settings" className="underline hover:text-red-300">
              Check AI settings
            </Link>
            .
          </div>
        )}

        <div ref={bottomRef} />
      </div>
    </div>
  );
}
