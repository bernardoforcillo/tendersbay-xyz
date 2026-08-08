import { cn, Pill, type PillProps } from '@tendersbay/components/core';
import { useTranslation } from 'react-i18next';

export type CoverageBadgeProps = {
  /** full | notice_only | none */
  coverage: string;
  /** "" | no_documents_published | notice_not_read | body_not_retrieved | not_yet_extracted | extraction_failed */
  reason: string;
  className?: string;
};

/**
 * How much of a tender's documents we hold, and why.
 *
 * Two stacked lines, never one. Line 1 is the QUANTITY, line 2 is the CAUSE, and
 * they are separate because they are orthogonal facts: "we hold only the notice
 * because the buyer published nothing else" and "we hold only the notice because
 * we have not fetched what they published" are the same quantity and opposite
 * situations — one is complete, the other is a gap in our work. A single-line
 * badge forces a choice between them and the product then says the wrong one.
 *
 * `no_documents_published` is the ONLY string in this product that asserts
 * absence. The backend only emits it when the notice was actually read and it
 * listed zero document links (`Availability.Consistent()` enforces the pairing),
 * so the claim is always earned.
 *
 * `tone="match"` is reserved for `coverage === 'full'`. Partial coverage never
 * borrows the confirmation color, and it never renders through the kit's
 * `EmptyState` either — an empty state reads as "nothing to see", which is the
 * exact conflation this component exists to prevent.
 */
export function CoverageBadge({ coverage, reason, className }: CoverageBadgeProps) {
  const { t } = useTranslation();
  const style = COVERAGE_STYLE[coverage] ?? { tone: 'neutral' as const, grayscale: true };
  const reasonStyle = reason ? REASON_TONE[reason] : undefined;

  return (
    <div className={cn('flex flex-col gap-1', className)}>
      <Pill
        tone={reasonStyle?.tone ?? style.tone}
        className={cn('w-fit', (reasonStyle?.grayscale ?? style.grayscale) && 'grayscale')}
      >
        {t(`tenders.coverage.label.${coverage}`, COVERAGE_FALLBACK[coverage] ?? coverage)}
      </Pill>
      {coverage !== 'full' && reason && (
        <p className="text-xs text-ink-500">
          {t(`tenders.coverage.reason.${reason}`, REASON_FALLBACK[reason] ?? reason)}
        </p>
      )}
    </div>
  );
}

type BadgeStyle = { tone: NonNullable<PillProps['tone']>; grayscale?: boolean };

const COVERAGE_STYLE: Record<string, BadgeStyle> = {
  full: { tone: 'match' },
  notice_only: { tone: 'neutral' },
  // There is no neutral-gray token in this theme (`ink` is green-tinted, `cream`
  // is warm), so an inactive look is the `grayscale` filter over a real tone.
  none: { tone: 'neutral', grayscale: true },
};

/**
 * A cause can override the quantity's tone, because a cause can be actionable
 * where the quantity is not: `body_not_retrieved` means the documents exist and
 * a human can open them at the buyer's URL right now, which is a deadline-shaped
 * nudge rather than a neutral statement. `extraction_failed` is the one genuine
 * failure — we tried and could not read the text.
 */
const REASON_TONE: Record<string, BadgeStyle> = {
  body_not_retrieved: { tone: 'deadline' },
  extraction_failed: { tone: 'urgent' },
  notice_not_read: { tone: 'neutral', grayscale: true },
};

const COVERAGE_FALLBACK: Record<string, string> = {
  full: 'Documents read',
  notice_only: 'Notice only',
  none: 'Nothing read',
};

const REASON_FALLBACK: Record<string, string> = {
  no_documents_published: 'The buyer published no documents with this notice.',
  notice_not_read:
    'We have not read this notice’s structured data, so we do not know what it publishes.',
  body_not_retrieved: 'The buyer publishes documents we have not retrieved yet.',
  not_yet_extracted: 'Queued — not read yet.',
  extraction_failed: 'We could not read the text of these documents.',
};
