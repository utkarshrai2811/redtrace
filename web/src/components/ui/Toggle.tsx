import { cn } from '../../lib/cn';

export interface ToggleProps {
  checked: boolean;
  onChange: (value: boolean) => void;
  label?: string;
  disabled?: boolean;
  tone?: 'accent' | 'green';
}

export function Toggle({ checked, onChange, label, disabled, tone = 'accent' }: ToggleProps) {
  const onColor = tone === 'green' ? 'bg-emerald-500' : 'bg-accent';
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={cn(
        'inline-flex items-center gap-2 select-none',
        disabled && 'opacity-40 pointer-events-none',
      )}
    >
      <span
        className={cn(
          'relative h-4 w-7 rounded-full transition-colors',
          checked ? onColor : 'bg-zinc-700',
        )}
      >
        <span
          className={cn(
            'absolute top-0.5 h-3 w-3 rounded-full bg-white transition-transform',
            checked ? 'translate-x-3.5' : 'translate-x-0.5',
          )}
        />
      </span>
      {label && <span className="text-xs text-zinc-300">{label}</span>}
    </button>
  );
}
