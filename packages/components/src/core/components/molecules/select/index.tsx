import type { SelectHTMLAttributes } from 'react';
import { cn } from '../../../cn';

export type SelectProps = Omit<SelectHTMLAttributes<HTMLSelectElement>, 'className'> & {
  label: string;
  className?: string;
};

export function Select({ label, className, id, children, ...props }: SelectProps) {
  return (
    <label className="flex flex-col gap-1.5">
      <span className="text-sm font-medium text-ink-700">{label}</span>
      <select
        {...props}
        id={id}
        className={cn(
          'h-10 rounded-xl border border-cream-300 bg-white px-3 text-sm text-ink-900 outline-none',
          'transition-colors duration-150 focus:border-brand-600 focus:ring-2 focus:ring-brand-600/25',
          'disabled:cursor-not-allowed disabled:opacity-50',
          className,
        )}
      >
        {children}
      </select>
    </label>
  );
}
