import { Card, cn, Pill } from '@tendersbay/components/core';
import { usePostHog } from 'posthog-js/react';
import { useState } from 'react';
import { Button } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import type { AnalyticsLocation } from '~/analytics';
import { bidClient } from '~/lib/api/client';

const OUTCOMES = ['won', 'lost', 'withdrawn'] as const;

export type BidOutcomeProps = {
  workbenchId: string;
  bidId: string;
  outcome: string;
  canManage: boolean;
  /** Analytics surface. */
  location: AnalyticsLocation;
  onRecorded: () => void;
  className?: string;
};

/**
 * How the bid ended. Lifted out of the bid detail page unchanged.
 *
 * Only rendered for a `go` bid — recording an outcome for something never bid on
 * is a category error, and the caller enforces it rather than this component
 * guessing.
 */
export function BidOutcome({
  workbenchId,
  bidId,
  outcome,
  canManage,
  location,
  onRecorded,
  className,
}: BidOutcomeProps) {
  const { t } = useTranslation();
  const posthog = usePostHog();
  const [busy, setBusy] = useState(false);

  async function record(next: (typeof OUTCOMES)[number]) {
    setBusy(true);
    try {
      await bidClient.recordOutcome({ workbenchId, bidId, outcome: next });
      posthog?.capture('bid_outcome_recorded', { location, outcome: next });
      onRecorded();
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card className={className}>
      <h2 className="text-sm font-semibold text-ink-700">
        {t('bid.detail.outcomeTitle', 'Outcome')}
      </h2>
      <div className="mt-3 flex flex-wrap items-center gap-2">
        {outcome ? (
          <Pill
            tone={outcome === 'won' ? 'match' : 'neutral'}
            className={cn(outcome !== 'won' && 'grayscale')}
          >
            {t(`bid.outcome.${outcome}`)}
          </Pill>
        ) : (
          canManage &&
          OUTCOMES.map((option) => (
            <Button
              key={option}
              isDisabled={busy}
              onPress={() => void record(option)}
              className={GHOST_BUTTON}
            >
              {t(`bid.outcome.${option}`)}
            </Button>
          ))
        )}
      </div>
    </Card>
  );
}

const GHOST_BUTTON =
  'inline-flex h-10 items-center justify-center rounded-xl border border-cream-300 bg-white px-4 text-sm font-semibold text-ink-800 outline-none ' +
  'transition-colors duration-150 data-[hovered]:border-cream-400 data-[pressed]:bg-cream-100 ' +
  'data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600';
