import { useState, type KeyboardEvent } from 'react';
import { Button } from '../ui/Button';
import { useAIStore } from '../../store/aiStore';

export function Composer() {
  const streaming = useAIStore((s) => s.streaming);
  const config = useAIStore((s) => s.config);
  const sendMessage = useAIStore((s) => s.sendMessage);

  const [value, setValue] = useState('');

  // Treat unloaded config as disabled so a send can't fire before we know AI is
  // enabled (which the backend would just reject with an error).
  const disabled = streaming || !config?.enabled;

  const submit = () => {
    const content = value.trim();
    if (!content || disabled) return;
    setValue('');
    void sendMessage(content);
  };

  const onKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      submit();
    }
  };

  return (
    <div className="flex shrink-0 items-end gap-2 border-t border-zinc-800 bg-panel/60 px-3 py-2.5">
      <textarea
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={onKeyDown}
        disabled={disabled}
        rows={2}
        spellCheck={false}
        aria-label="Message"
        placeholder={
          config && !config.enabled
            ? 'AI is disabled — configure a provider in Settings'
            : 'Ask anything… (Enter to send, Shift+Enter for a newline)'
        }
        className="min-h-[2.5rem] flex-1 resize-y rounded-md border border-zinc-700 bg-zinc-900 px-2.5 py-1.5 text-xs text-zinc-200 placeholder:text-zinc-500 focus:border-accent/60 focus:outline-none focus:ring-1 focus:ring-accent/40 disabled:opacity-60"
      />
      <Button
        variant="primary"
        size="sm"
        disabled={disabled || value.trim().length === 0}
        onClick={submit}
        className="shrink-0"
      >
        {streaming ? 'Sending…' : 'Send'}
      </Button>
    </div>
  );
}
