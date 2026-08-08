import { Card, cn } from '@tendersbay/components/core';
import { usePostHog } from 'posthog-js/react';
import { useState } from 'react';
import { Button } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import type { AnalyticsLocation } from '~/analytics';
import { bidClient } from '~/lib/api/client';

export const BID_STAGES = ['shortlisted', 'preparing', 'submitted'] as const;

export type BidStageStepperProps = {
  workbenchId: string;
  bidId: string;
  stage: string;
  /** A no-go bid may not be advanced — but the control still explains itself. */
  isNoGo: boolean;
  canManage: boolean;
  /** Analytics surface. */
  location: AnalyticsLocation;
  onAdvanced: () => void;
  className?: string;
};

/**
 * Where this bid is in its lifecycle, and the one control that moves it.
 *
 * The advance button under a no-go is `aria-disabled`, NOT `isDisabled`.
 * `isDisabled` drops a control out of the tab order and takes its explanation
 * with it: a keyboard or screen-reader user then meets a button that has silently
 * vanished, with no way to learn why. Keeping it focusable and announcing it as
 * unavailable is the difference between "you can't" and "you can't, and here is
 * the reason".
 */
export function BidStageStepper({
  workbenchId,
  bidId,
  stage,
  isNoGo,
  canManage,
  location,
  onAdvanced,
  className,
}: BidStageStepperProps) {
  const { t } = useTranslation();
  const posthog = usePostHog();
  const [busy, setBusy] = useState(false);

  const stageIndex = BID_STAGES.indexOf(stage as (typeof BID_STAGES)[number]);

  async function advance() {
    setBusy(true);
    try {
      await bidClient.advanceStage({ workbenchId, bidId });
      posthog?.capture('bid_stage_changed', { location, from_stage: stage });
      onAdvanced();
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card className={className}>
      <h2 className="text-sm font-semibold text-ink-700">{t('bid.detail.stageTitle', 'Stage')}</h2>
      <ol className="mt-3 flex items-center gap-2">
        {BID_STAGES.map((s, i) => (
          <li
            key={s}
            className={cn(
              'rounded-full px-3 py-1 text-xs font-medium',
              i <= stageIndex ? 'bg-brand-100 text-brand-700' : 'bg-cream-100 text-ink-400',
            )}
          >
            {t(`bid.stage.${s}`)}
          </li>
        ))}
      </ol>
      {canManage &&
        stage !== 'submitted' &&
        (isNoGo ? (
          <div className="mt-3 flex flex-col gap-1">
            <Button
              aria-disabled="true"
              onPress={() => {}}
              className={cn(PRIMARY_BUTTON, 'cursor-default opacity-50')}
            >
              {t('bid.actions.advance', 'Advance stage')}
            </Button>
            <p className="text-xs text-ink-500">
              {t('bid.actions.advanceBlockedByNoGo', 'This bid is marked no-go.')}
            </p>
          </div>
        ) : (
          <Button
            isDisabled={busy}
            onPress={() => void advance()}
            className={cn(PRIMARY_BUTTON, 'mt-3')}
          >
            {t('bid.actions.advance', 'Advance stage')}
          </Button>
        ))}
    </Card>
  );
}

const PRIMARY_BUTTON =
  'inline-flex h-10 w-fit items-center justify-center rounded-xl bg-brand-600 px-4 text-sm font-semibold text-white outline-none ' +
  'transition-colors duration-150 data-[hovered]:bg-brand-700 data-[pressed]:bg-brand-800 ' +
  'data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600 data-[focus-visible]:ring-offset-2';
