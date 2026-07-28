import type { ReactNode } from 'react';
import { cn } from '../../../cn';

// 'warning' is for a result that is usable but qualified — a search that ran
// on one retriever instead of two, say. It is deliberately distinct from
// 'error': telling someone their results are incomplete is not the same as
// telling them the request failed.
type Tone = 'error' | 'warning' | 'success';

export type BannerProps = {
  tone: Tone;
  children: ReactNode;
  className?: string;
};

const ROLE: Record<Tone, 'alert' | 'status'> = {
  error: 'alert',
  // 'status' rather than 'alert': a qualified result is worth announcing
  // politely, not worth interrupting a screen reader mid-sentence for.
  warning: 'status',
  success: 'status',
};

const TONES: Record<Tone, string> = {
  error: 'border-red-200 bg-red-50 text-red-700',
  warning: 'border-amber-200 bg-amber-50 text-amber-800',
  success: 'border-brand-200 bg-brand-50 text-brand-800',
};

export function Banner({ tone, children, className }: BannerProps) {
  return (
    <div
      role={ROLE[tone]}
      className={cn('rounded-xl border px-4 py-3 text-sm', TONES[tone], className)}
    >
      {children}
    </div>
  );
}
