import { cn, Pill, type PillProps } from '@tendersbay/components/core';
import { forwardRef } from 'react';
import { useTranslation } from 'react-i18next';

export type GapStatusPillProps = {
  /** met | missing | shortfall | expiring | expired | unknown */
  status: string;
  /** Whether this gap actually blocks admission, as decided by the domain. */
  blocking?: boolean;
  className?: string;
};

/**
 * One gap's status, coloured by what it costs the bidder.
 *
 * The mapping is deliberately not one tone per status. `missing` on a
 * *non-blocking* criterio premiante is a lost point, not a lost gara, and
 * painting it the same urgent red as a missing requisito di partecipazione
 * trains the reader to ignore red. Blocking-ness, not status alone, is what
 * earns the urgent tone.
 *
 * `unknown` is grayscale rather than a gray token because this theme has none —
 * `ink` is green-tinted and `cream` is warm. It reads as "not yet answered",
 * which is exactly what it is: nobody has been asked.
 *
 * It forwards a ref because focus moves here after a just-in-time answer lands.
 * The pill is the thing that changed, and without moving focus to it the answer
 * lands silently for a screen-reader user.
 */
export const GapStatusPill = forwardRef<HTMLSpanElement, GapStatusPillProps>(function GapStatusPill(
  { status, blocking = false, className },
  ref,
) {
  const { t } = useTranslation();
  const style = toneFor(status, blocking);
  return (
    // The focus target is a wrapper rather than the kit `Pill` itself: `Pill`'s
    // props are plain span attributes with no ref in the type, and widening a
    // shared kit primitive for one feature's focus management is the wrong
    // trade. `tabIndex={-1}` keeps it out of the tab order while remaining a
    // valid programmatic focus target.
    <span ref={ref} tabIndex={-1} className={cn('inline-flex outline-none', className)}>
      <Pill tone={style.tone} className={cn('w-fit', style.grayscale && 'grayscale')}>
        {t(`company.eligibility.status.${status}`, STATUS_FALLBACK[status] ?? status)}
      </Pill>
    </span>
  );
});

type PillStyle = { tone: NonNullable<PillProps['tone']>; grayscale?: boolean };

function toneFor(status: string, blocking: boolean): PillStyle {
  if (status === 'met') return { tone: 'match' };
  if (status === 'unknown') return { tone: 'neutral', grayscale: true };
  if (status === 'expiring') return { tone: 'deadline' };
  if (blocking && (status === 'missing' || status === 'shortfall' || status === 'expired')) {
    return { tone: 'urgent' };
  }
  return { tone: 'neutral' };
}

const STATUS_FALLBACK: Record<string, string> = {
  met: 'Met',
  missing: 'Missing',
  shortfall: 'Short of the threshold',
  expiring: 'Expiring',
  expired: 'Expired',
  unknown: 'Not answered yet',
};
