import { Link } from 'react-router-dom';

/**
 * Shown when the AI assistant is not configured. Explains that a provider and
 * API key must be set in Settings before conversations can generate replies.
 */
export function AIDisabledNotice() {
  return (
    <div className="flex min-h-0 flex-1 items-center justify-center px-6 py-10">
      <div className="max-w-lg space-y-3 rounded-lg border border-zinc-800 bg-panel/50 px-5 py-5 text-center">
        <div className="mx-auto flex h-9 w-9 items-center justify-center rounded-full border border-zinc-700 text-zinc-500">
          <svg
            width="18"
            height="18"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.8"
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden="true"
          >
            <path d="M12 3l2 6 6 2-6 2-2 6-2-6-6-2 6-2z" />
          </svg>
        </div>
        <h2 className="text-sm font-semibold text-zinc-100">AI assistant is disabled</h2>
        <p className="text-xs leading-relaxed text-zinc-400">
          No AI provider or API key is configured. Set a provider, model, and key in{' '}
          <Link to="/settings" className="text-accent-fg underline hover:text-accent">
            Settings
          </Link>{' '}
          to enable AI-assisted analysis, triage, and payload suggestions.
        </p>
      </div>
    </div>
  );
}
