import { forwardRef, type ButtonHTMLAttributes } from 'react';
import { cn } from '../../lib/cn';

type Variant = 'default' | 'primary' | 'danger' | 'ghost' | 'outline';
type Size = 'sm' | 'md' | 'icon';

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: Size;
}

const base =
  'inline-flex items-center justify-center gap-1.5 rounded-md font-medium select-none ' +
  'transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent/60 ' +
  'disabled:opacity-40 disabled:pointer-events-none';

const variants: Record<Variant, string> = {
  default: 'bg-zinc-800 text-zinc-100 hover:bg-zinc-700 border border-zinc-700',
  primary: 'bg-accent text-white hover:bg-red-600 border border-red-600/40 shadow-sm',
  danger: 'bg-transparent text-red-400 hover:bg-red-500/10 border border-red-500/30',
  ghost: 'bg-transparent text-zinc-300 hover:bg-zinc-800 hover:text-zinc-100',
  outline: 'bg-transparent text-zinc-200 border border-zinc-700 hover:bg-zinc-800',
};

const sizes: Record<Size, string> = {
  sm: 'h-7 px-2.5 text-xs',
  md: 'h-8 px-3 text-sm',
  icon: 'h-7 w-7 text-sm',
};

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { variant = 'default', size = 'md', className, type, ...props },
  ref,
) {
  return (
    <button
      ref={ref}
      type={type ?? 'button'}
      className={cn(base, variants[variant], sizes[size], className)}
      {...props}
    />
  );
});
