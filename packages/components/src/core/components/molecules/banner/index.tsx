import type { ReactNode } from 'react';
import { cn } from '../../../cn';

type Tone = 'error' | 'success';

export type BannerProps = {
  tone: Tone;
  children: ReactNode;
  className?: string;
};

const ROLE: Record<Tone, 'alert' | 'status'> = {
  error: 'alert',
  success: 'status',
};

const TONES: Record<Tone, string> = {
  error: 'border-red-200 bg-red-50 text-red-700',
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
