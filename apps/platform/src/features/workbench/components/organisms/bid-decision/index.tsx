import { Card, cn, Pill } from '@tendersbay/components/core';
import { useState } from 'react';
import { Button } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import type { AnalyticsLocation, ReportedVerdict } from '~/analytics';
import { useCaptureEvent } from '~/analytics';
import { EligibilityVerdict } from '~/features/company/components/organisms';
import type { AssessmentView } from '~/features/company/constants';
import { bidClient } from '~/lib/api/client';

/**
 * The persisted decision record.
 *
 * Every field is optional because they are Phase 4 transport additions; a bid
 * that has not been decided yet legitimately carries none of them, and so does a
 * generated message that predates the fields. `decisionRecommendation === ''`
 * therefore means one specific thing — no eligibility check was available when
 * the human decided — which is NOT the same as `insufficient_data` (the check ran
 * and the evidence was thin). Collapsing the two makes the override rate
 * un-interpretable.
 */
export type BidDecisionView = {
  goNoGo: string;
  decisionRecommendation?: string;
  decisionOverridden?: boolean;
  decisionBlockingGapCount?: number;
  decisionRecordedAt?: string;
};

export type BidDecisionProps = {
  workbenchId: string;
  bidId: string;
  bid: BidDecisionView;
  canManage: boolean;
  assessment: AssessmentView | null;
  /** Analytics surface. */
  location: AnalyticsLocation;
  /** Refetch the bid so the recorded baseline appears. */
  onDecided: () => void;
  className?: string;
};

/**
 * The go/no-go, with the recommendation above it and the override recorded below.
 *
 * Three deliberate absences:
 *
 *  - **No confirmation dialog on overriding a no_go.** Nothing may refuse a go on
 *    a no-go recommendation — that is the entire point of an override being
 *    representable rather than prevented. A human with local knowledge (an
 *    avvalimento already agreed, a classifica about to be issued) is often right.
 *  - **No required justification field.** It would depress exactly the rate
 *    `go_no_go_overridden` exists to measure, and a mandatory free-text box
 *    collects "n/a" at scale.
 *  - **No confidence band.** See `eligibility-verdict`.
 *
 * What IS recorded, and shown back, is the baseline the human decided against.
 * That is what makes an override auditable later without making it harder now.
 */
export function BidDecision({
  workbenchId,
  bidId,
  bid,
  canManage,
  assessment,
  location,
  onDecided,
  className,
}: BidDecisionProps) {
  const { t } = useTranslation();
  const capture = useCaptureEvent();
  const [busy, setBusy] = useState(false);

  const isNoGo = bid.goNoGo === 'no_go';

  async function decide(decision: 'go' | 'no_go') {
    setBusy(true);
    try {
      const res = await bidClient.setGoNoGo({ workbenchId, bidId, goNoGo: decision });
      // SetGoNoGo returns the written Bid, so the recorded baseline is read from
      // the response rather than guessed from the assessment the client happened
      // to be holding — the two can differ if the dossier moved mid-session.
      const recorded: BidDecisionView | undefined = res.bid;
      capture('bid_decision_recorded', {
        location,
        decision,
        recommendation: (recorded?.decisionRecommendation ?? '') as ReportedVerdict,
        overridden: Boolean(recorded?.decisionOverridden),
      });
      if (recorded?.decisionOverridden) {
        capture('go_no_go_overridden', {
          location,
          from: (recorded.decisionRecommendation ?? '') as ReportedVerdict,
          to: decision,
          blocking_gap_count: recorded.decisionBlockingGapCount ?? 0,
        });
      }
      onDecided();
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card className={className}>
      <h2 className="text-sm font-semibold text-ink-700">
        {t('bid.detail.goNoGoTitle', 'Decision')}
      </h2>

      {/* The recommendation sits ABOVE the buttons: a decision surface that shows
          the verdict after the controls invites the click before the reading. */}
      {assessment && (
        <EligibilityVerdict assessment={assessment} location={location} className="mt-3" />
      )}

      <div className="mt-4 flex flex-wrap items-center gap-2">
        <Pill
          tone={bid.goNoGo === 'go' ? 'match' : 'neutral'}
          className={cn(isNoGo && 'grayscale')}
        >
          {t(`bid.goNoGo.${bid.goNoGo === 'no_go' ? 'noGo' : bid.goNoGo}`)}
        </Pill>
        {canManage && (
          <>
            <Button isDisabled={busy} onPress={() => void decide('go')} className={GHOST_BUTTON}>
              {t('bid.actions.markGo')}
            </Button>
            <Button isDisabled={busy} onPress={() => void decide('no_go')} className={GHOST_BUTTON}>
              {t('bid.actions.markNoGo', 'No-go')}
            </Button>
          </>
        )}
      </div>

      <RecordedBaseline bid={bid} />
    </Card>
  );
}

/** What the human decided against, in the three states the record can be in. */
function RecordedBaseline({ bid }: { bid: BidDecisionView }) {
  const { t } = useTranslation();
  if (!bid.decisionRecordedAt) return null;

  const recommendation = bid.decisionRecommendation ?? '';
  // Labelled, not bare. A date appended to a fragment list reads as a second
  // datum about the decision rather than as when it was taken, and it is the
  // only fragment here with no word attached to it in any of the 24 locales.
  const when = t('bid.decision.recordedAt', {
    when: formatDate(bid.decisionRecordedAt),
    defaultValue: 'recorded {{when}}',
  });

  if (!recommendation) {
    return (
      <p className="mt-3 text-xs text-ink-500">
        {t('bid.decision.noRecommendation', 'No eligibility check was available when you decided.')}{' '}
        {when}
      </p>
    );
  }

  const fragments = [
    t('bid.decision.recordedAgainst', {
      recommendation: t(`company.eligibility.verdict.${recommendation}`, recommendation),
      defaultValue: 'Recorded against: {{recommendation}}',
    }),
    t('bid.decision.blockingGapsAtDecision', {
      count: bid.decisionBlockingGapCount ?? 0,
      defaultValue_one: '{{count}} blocking gap',
      defaultValue_other: '{{count}} blocking gaps',
    }),
  ];
  if (bid.decisionOverridden) {
    fragments.push(
      t('bid.decision.youDecided', {
        decision: t(`bid.goNoGo.${bid.goNoGo === 'no_go' ? 'noGo' : bid.goNoGo}`),
        defaultValue: 'you decided {{decision}}',
      }),
    );
  }
  fragments.push(when);

  return (
    <p className={cn('mt-3 text-xs', bid.decisionOverridden ? 'text-ink-700' : 'text-ink-500')}>
      {fragments.filter(Boolean).join(' · ')}
    </p>
  );
}

function formatDate(rfc3339: string): string {
  const date = new Date(rfc3339);
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleDateString();
}

const GHOST_BUTTON =
  'inline-flex h-10 items-center justify-center rounded-xl border border-cream-300 bg-white px-4 text-sm font-semibold text-ink-800 outline-none ' +
  'transition-colors duration-150 data-[hovered]:border-cream-400 data-[pressed]:bg-cream-100 ' +
  'data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600';
