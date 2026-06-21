import { useEffect, useRef, useState } from 'react';
import { cn } from '../../lib/cn';

interface CopyButtonProps {
  /** The text written to the clipboard. */
  value: string;
  /** Accessible label, e.g. "Copy HTTP URL". */
  label: string;
  className?: string;
}

/**
 * A small glyph button that copies `value` to the clipboard and briefly shows a
 * "Copied" affordance. Always carries an aria-label since it is icon-only.
 */
export function CopyButton({ value, label, className }: CopyButtonProps) {
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
    },
    [],
  );

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      if (timer.current) clearTimeout(timer.current);
      timer.current = setTimeout(() => setCopied(false), 1200);
    } catch {
      // Clipboard may be unavailable (insecure context); fail quietly.
    }
  };

  return (
    <button
      type="button"
      onClick={() => void copy()}
      aria-label={copied ? `${label} (copied)` : label}
      title={copied ? 'Copied' : 'Copy'}
      className={cn(
        'inline-flex shrink-0 items-center gap-1 rounded px-1.5 py-0.5 text-2xs text-zinc-400 transition-colors',
        'hover:bg-zinc-800 hover:text-zinc-200',
        'focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent/60',
        copied && 'text-emerald-400',
        className,
      )}
    >
      {copied ? (
        <>
          <svg
            width="12"
            height="12"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2.4"
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden="true"
          >
            <path d="M20 6L9 17l-5-5" />
          </svg>
          Copied
        </>
      ) : (
        <>
          <svg
            width="12"
            height="12"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden="true"
          >
            <rect x="9" y="9" width="13" height="13" rx="2" />
            <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1" />
          </svg>
          Copy
        </>
      )}
    </button>
  );
}
