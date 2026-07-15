import { cn } from '@tendersbay/components/core';

export function Logo({ className }: { className?: string }) {
  return (
    <span className={cn('text-lg font-extrabold tracking-tight text-ink-900', className)}>
      tenders<span className="text-brand-600">bay</span>
    </span>
  );
}
