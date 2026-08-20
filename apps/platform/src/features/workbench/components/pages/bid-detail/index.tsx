import { Link, useParams } from '@tanstack/react-router';
import { Banner, Card, cn, Pill } from '@tendersbay/components/core';
import type { Bid } from '@tendersbay/proto/bid/v1/bid_pb';
import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import type { AnalyticsLocation } from '~/analytics';
import {
  deadlineInfo,
  type FitTier,
  fitReasonFragments,
  fitTierPillClassName,
  fitTierPillTone,
} from '~/features/account/components/organisms/tender-feed';
import { EligibilityGapList } from '~/features/company/components/organisms';
import { useEligibility } from '~/features/company/hooks';
import { AwardCriteriaGrid, TenderCoverage } from '~/features/tenders/components/organisms';
import type { TenderCriteriaSource } from '~/features/tenders/constants';
import { useTenderAvailability, useTenderDetail } from '~/features/tenders/hooks';
import { BidChecklist } from '~/features/workbench/components/organisms/bid-checklist';
import { BidDecision } from '~/features/workbench/components/organisms/bid-decision';
import { BidOutcome } from '~/features/workbench/components/organisms/bid-outcome';
import { BidRepairChat } from '~/features/workbench/components/organisms/bid-repair-chat';
import { BidStageStepper } from '~/features/workbench/components/organisms/bid-stage-stepper';
import { SchedaGara } from '~/features/workbench/components/templates/scheda-gara';
import { useWorkbenchContext } from '~/features/workbench/context';
import { useBid, useChecklist } from '~/features/workbench/hooks';
import { useReminderAttribution } from '~/features/workbench/hooks/use-reminder-attribution';
import { can, Permission } from '~/features/workbench/permissions';
import { useSchedaGaraStore } from '~/store/scheda-gara';

const FIT_TIER_KEY: Record<FitTier, string> = {
  strong: 'tenders.fit.tier.strong',
  possible: 'tenders.fit.tier.possible',
  long_shot: 'tenders.fit.tier.longShot',
};

// Typed against the catalogue's closed set rather than inferred as a string, so
// a surface renamed there fails HERE at compile time instead of silently
// scrubbing every event this page fires into the `invalid` bucket.
const LOCATION: AnalyticsLocation = 'bid_detail';

/**
 * The bid detail route: data loading and composition only.
 *
 * Everything that used to be rendered inline here now lives in an organism, and
 * the order those organisms appear in lives in the `SchedaGara` template. What is
 * left is the wiring: five reads (bid, checklist, eligibility, coverage, tender
 * detail), the permission check, and the refetches that make an answered question
 * visibly move the verdict.
 */
export function BidDetailPage() {
  const { t } = useTranslation();
  const { bidId, workspaceId, workbenchId } = useParams({
    from: '/_authenticated/workspaces/$workspaceId/workbench/$workbenchId/bids/$bidId',
  });
  // Attribute the visit if a deadline reminder sent them here, before anything
  // else touches the URL.
  useReminderAttribution(bidId);

  const { myPermissions } = useWorkbenchContext();
  const canManage = can(myPermissions, Permission.ManageWorkbench);

  const { data: bid, loading, error, refetch: refetchBid } = useBid(bidId);
  const { data: items, refetch: refetchChecklist } = useChecklist(bidId);

  const tenderId = bid?.tenderId ?? '';
  const { data: assessment, refetch: refetchAssessment } = useEligibility(workspaceId, tenderId);
  const { data: availability } = useTenderAvailability(tenderId);
  const { data: tender } = useTenderDetail(tenderId);

  // Whether the repair panel is expanded is per-gara UI state; carrying it from
  // one bid to the next would open a panel about a gara the reader has left.
  const resetSchedaGara = useSchedaGaraStore((s) => s.reset);
  // biome-ignore lint/correctness/useExhaustiveDependencies: bidId is the reset TRIGGER, not a value the effect reads — the route reuses this component across bids, so without it an open repair panel would survive the change
  useEffect(() => {
    resetSchedaGara();
    return resetSchedaGara;
  }, [bidId, resetSchedaGara]);

  if (loading)
    return <p className="text-sm text-ink-500">{t('workbench.common.loading', 'Loading…')}</p>;
  if (error || !bid)
    return (
      <Banner tone="error">
        {error ?? t('bid.errors.generic', 'Something went wrong. Please try again.')}
      </Banner>
    );

  function refetchAfterCapture() {
    refetchAssessment();
    refetchBid();
  }

  const criteriaSource: TenderCriteriaSource | null = tender;

  return (
    <div className="flex flex-col gap-4">
      <Link
        to="/workspaces/$workspaceId/workbench/$workbenchId"
        params={{ workspaceId, workbenchId }}
        className="text-sm text-brand-700 no-underline hover:underline"
      >
        {t('bid.detail.backToPortfolio', 'Back to portfolio')}
      </Link>

      <SchedaGara
        header={<BidHeader bid={bid} />}
        coverage={
          availability ? (
            <TenderCoverage
              availability={availability}
              documentsUrl={criteriaSource?.documentsUrl}
              location={LOCATION}
            />
          ) : undefined
        }
        decision={
          <BidDecision
            workbenchId={workbenchId}
            bidId={bidId}
            bid={bid}
            canManage={canManage}
            assessment={assessment}
            location={LOCATION}
            onDecided={refetchBid}
          />
        }
        gaps={
          assessment ? (
            <Card>
              <h2 className="text-sm font-semibold text-ink-700">
                {t('company.requirements.title', 'Participation requirements')}
              </h2>
              <EligibilityGapList
                className="mt-3"
                workspaceId={workspaceId}
                tenderId={tenderId}
                assessment={assessment}
                location={LOCATION}
                onCaptured={refetchAfterCapture}
              />
            </Card>
          ) : undefined
        }
        criteria={
          criteriaSource ? (
            <AwardCriteriaGrid tender={criteriaSource} location={LOCATION} />
          ) : undefined
        }
        checklist={
          <BidChecklist
            workbenchId={workbenchId}
            bidId={bidId}
            items={items ?? []}
            done={bid.checklistDone}
            total={bid.checklistTotal}
            canManage={canManage}
            location={LOCATION}
            onChanged={() => {
              refetchChecklist();
              refetchBid();
            }}
          />
        }
        stage={
          <BidStageStepper
            workbenchId={workbenchId}
            bidId={bidId}
            stage={bid.stage}
            isNoGo={bid.goNoGo === 'no_go'}
            canManage={canManage}
            location={LOCATION}
            onAdvanced={refetchBid}
          />
        }
        outcome={
          bid.goNoGo === 'go' ? (
            <BidOutcome
              workbenchId={workbenchId}
              bidId={bidId}
              outcome={bid.outcome}
              canManage={canManage}
              location={LOCATION}
              onRecorded={refetchBid}
            />
          ) : undefined
        }
        repairChat={<BidRepairChat workbenchId={workbenchId} location={LOCATION} />}
      />
    </div>
  );
}

/**
 * Picked off the wire message rather than re-declared: `reason` is a structured
 * `ReasonSignals`, and restating it as a string here would quietly diverge from
 * what `fitReasonFragments` actually parses.
 */
type BidHeaderData = Pick<
  Bid,
  | 'tenderAvailable'
  | 'tenderTitle'
  | 'tenderBuyerName'
  | 'tenderDeadline'
  | 'fitTier'
  | 'reason'
  | 'needsProfile'
>;

/** Which gara this is, how well it fits, and when it closes — or its tombstone. */
function BidHeader({ bid }: { bid: BidHeaderData }) {
  const { t } = useTranslation();

  if (!bid.tenderAvailable) {
    return (
      <Card className="border-cream-300 bg-cream-50">
        <h1 className="text-lg font-semibold text-ink-900">{t('bid.tombstone.title')}</h1>
        <p className="mt-1 text-sm text-ink-500">
          {t(
            'bid.tombstone.description',
            'The source portal removed this listing. Your checklist and notes are preserved.',
          )}
        </p>
      </Card>
    );
  }

  const deadline = deadlineInfo(bid.tenderDeadline, new Date());
  const tier = bid.fitTier ? (bid.fitTier as FitTier) : null;

  return (
    <header className="space-y-2">
      <div className="flex flex-wrap items-center gap-2">
        {tier && (
          <Pill tone={fitTierPillTone(tier)} className={cn(fitTierPillClassName(tier))}>
            {t(FIT_TIER_KEY[tier])}
          </Pill>
        )}
        {deadline && (
          <Pill tone={deadline.tone} className="ml-auto shrink-0">
            {deadline.days < 0
              ? t('tenders.deadline.expired')
              : deadline.days === 0
                ? t('tenders.deadline.today')
                : t('tenders.deadline.days', { count: deadline.days })}
          </Pill>
        )}
      </div>
      <h1 className="text-2xl font-semibold leading-snug text-ink-900">{bid.tenderTitle}</h1>
      {bid.tenderBuyerName && <p className="text-sm text-ink-500">{bid.tenderBuyerName}</p>}
      {bid.needsProfile ? (
        <p className="text-sm text-ink-500">
          {t('bid.fit.needsProfile', 'Set up a client profile to see fit')}
        </p>
      ) : (
        tier &&
        bid.reason && (
          <p className="text-xs text-ink-500">
            {fitReasonFragments(bid.reason)
              .map((f) => t(f.key, { count: f.count }))
              .join(' · ')}
          </p>
        )
      )}
    </header>
  );
}
