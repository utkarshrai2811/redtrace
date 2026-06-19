import { forwardRef, type InputHTMLAttributes } from 'react';
import { cn } from '../../lib/cn';

export type InputProps = InputHTMLAttributes<HTMLInputElement>;

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input(
  { className, ...props },
  ref,
) {
  return (
    <input
      ref={ref}
      className={cn(
        'h-7 rounded-md border border-zinc-700 bg-zinc-900 px-2 text-xs text-zinc-200',
        'placeholder:text-zinc-500',
        'focus:border-accent/60 focus:outline-none focus:ring-1 focus:ring-accent/40',
        className,
      )}
      {...props}
    />
  );
});
