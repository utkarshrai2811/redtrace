import { forwardRef, type SelectHTMLAttributes } from 'react';
import { cn } from '../../lib/cn';

export type SelectProps = SelectHTMLAttributes<HTMLSelectElement>;

export const Select = forwardRef<HTMLSelectElement, SelectProps>(function Select(
  { className, children, ...props },
  ref,
) {
  return (
    <select
      ref={ref}
      className={cn(
        'h-7 rounded-md border border-zinc-700 bg-zinc-900 px-2 text-xs text-zinc-200',
        'focus:border-accent/60 focus:outline-none focus:ring-1 focus:ring-accent/40',
        'appearance-none bg-[length:14px] bg-[right_0.4rem_center] bg-no-repeat pr-6',
        className,
      )}
      style={{
        backgroundImage:
          "url(\"data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='%2371717a' stroke-width='2'><path d='M6 9l6 6 6-6'/></svg>\")",
      }}
      {...props}
    >
      {children}
    </select>
  );
});
