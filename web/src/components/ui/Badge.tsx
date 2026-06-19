import { type HTMLAttributes } from 'react';
import { cn } from '../../lib/cn';

type Tone = 'zinc' | 'green' | 'amber' | 'red' | 'blue' | 'accent';

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  tone?: Tone;
}

const tones: Record<Tone, string> = {
  zinc: 'bg-zinc-800 text-zinc-300 border-zinc-700',
  green: 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30',
  amber: 'bg-amber-500/10 text-amber-400 border-amber-500/30',
  red: 'bg-red-500/10 text-red-400 border-red-500/30',
  blue: 'bg-sky-500/10 text-sky-400 border-sky-500/30',
  accent: 'bg-accent/15 text-accent-fg border-accent/40',
};

export function Badge({ tone = 'zinc', className, ...props }: BadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded border px-1.5 py-0.5 text-2xs font-medium leading-none',
        tones[tone],
        className,
      )}
      {...props}
    />
  );
}
