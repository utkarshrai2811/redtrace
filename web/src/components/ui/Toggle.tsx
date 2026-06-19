import { cn } from '../../lib/cn';

export interface ToggleProps {
  checked: boolean;
  onChange: (value: boolean) => void;
  label?: string;
  /** Accessible name when there is no visible label (e.g. row toggles). */
  ariaLabel?: string;
  disabled?: boolean;
  tone?: 'accent' | 'green';
}

export function Toggle({ checked, onChange, label, ariaLabel, disabled, tone = 'accent' }: ToggleProps) {
  const onColor = tone === 'green' ? 'bg-emerald-500' : 'bg-accent';
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={ariaLabel ?? label}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={cn(
        'inline-flex items-center gap-2 rounded select-none',
        'focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent/60 focus-visible:ring-offset-1 focus-visible:ring-offset-canvas',
        disabled && 'opacity-40 pointer-events-none',
      )}
    >
      <span
        className={cn(
          'relative h-4 w-7 shrink-0 rounded-full transition-colors',
          checked ? onColor : 'bg-zinc-700',
        )}
      >
        <span
          className={cn(
            'absolute left-0.5 top-0.5 h-3 w-3 rounded-full bg-white transition-transform',
            checked ? 'translate-x-3' : 'translate-x-0',
          )}
        />
      </span>
      {label && (
        // min-width keeps "On"/"Off" from changing the layout on toggle; longer
        // static labels simply exceed it.
        <span className="inline-block min-w-[1.75rem] text-left text-xs text-zinc-300">{label}</span>
      )}
    </button>
  );
}
