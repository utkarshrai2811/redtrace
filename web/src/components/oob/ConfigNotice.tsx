function Flag({ children }: { children: string }) {
  return (
    <code className="rounded bg-zinc-800 px-1 py-0.5 font-mono text-2xs text-zinc-200">
      {children}
    </code>
  );
}

/**
 * Shown when the OOB listeners are not configured. Explains that the
 * Collaborator is off and how to start RedTrace with it enabled.
 */
export function ConfigNotice() {
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
            <path d="M5 12a7 7 0 1014 0 7 7 0 00-14 0zM12 2v3M12 19v3M2 12h3M19 12h3" />
          </svg>
        </div>
        <h2 className="text-sm font-semibold text-zinc-100">Collaborator is disabled</h2>
        <p className="text-xs leading-relaxed text-zinc-400">
          Out-of-band interaction capture is off because no listener domain or public IP is
          configured. Start RedTrace with <Flag>--oob-domain &lt;your-domain&gt;</Flag> and{' '}
          <Flag>--oob-public-ip &lt;your-ip&gt;</Flag> (and point that domain&apos;s DNS/NS at this
          host) to capture out-of-band callbacks.
        </p>
      </div>
    </div>
  );
}
