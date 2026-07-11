import type { HTMLAttributes } from 'react';
import { cn } from '../../../cn';

type Tone = 'match' | 'deadline' | 'urgent' | 'neutral';

export type PillProps = HTMLAttributes<HTMLSpanElement> & {
  /** One color = one meaning: match/confirmation, deadline approaching, urgent, or neutral count. */
  tone?: Tone;
};

const TONES: Record<Tone, string> = {
  match: 'bg-brand-100 text-brand-700',
  deadline: 'bg-signal-warm-100 text-signal-warm-700',
  urgent: 'bg-signal-urgent-100 text-signal-urgent-700',
  neutral: 'bg-cream-200 text-cream-900',
};

export function Pill({ tone = 'neutral', className, ...props }: PillProps) {
  return (
    <span
      {...props}
      className={cn(
        'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-semibold',
        TONES[tone],
        className,
      )}
    />
  );
}
