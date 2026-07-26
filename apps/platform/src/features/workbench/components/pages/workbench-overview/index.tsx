import { Link } from '@tanstack/react-router';
import { Card, cn, EmptyState, Pill } from '@tendersbay/components/core';
import type { Bid } from '@tendersbay/proto/bid/v1/bid_pb';
import { useTranslation } from 'react-i18next';
import { TenderResultCard } from '~/features/account/components/organisms';
import {
  deadlineInfo,
  type FitTier,
  fitTierPillClassName,
  fitTierPillTone,
} from '~/features/account/components/organisms/tender-feed';
import { useClientShortlist } from '~/features/account/components/pages/explore/use-client-shortlist';
import { useTenderLink } from '~/features/tenders';
import { WorkbenchChatPanel } from '~/features/workbench/components/organisms/workbench-chat-panel';
import { useWorkbenchContext } from '~/features/workbench/context';
import { useBids } from '~/features/workbench/hooks';

type Band = 'needsYouNow' | 'active' | 'submitted' | 'closed';
const BAND_ORDER: Band[] = ['needsYouNow', 'active', 'submitted', 'closed'];
const NEEDS_YOU_NOW_DAYS = 14;

function bandOf(bid: Bid, now: Date): Band {
  if (bid.outcome || bid.goNoGo === 'no_go') return 'closed';
  const dl = bid.tenderAvailable ? deadlineInfo(bid.tenderDeadline, now) : null;
  const incomplete = bid.checklistDone < bid.checklistTotal;
  if (dl && dl.days >= 0 && dl.days <= NEEDS_YOU_NOW_DAYS && incomplete) return 'needsYouNow';
  if (bid.stage === 'submitted') return 'submitted';
  return 'active';
}

export function WorkbenchOverviewPage() {
  const { t } = useTranslation();
  const { workbench } = useWorkbenchContext();
  const workspaceId = workbench.workspaceId;
  const { data: bids, loading } = useBids(workbench.id);

  if (loading)
    return (
      <>
        <p className="text-sm text-ink-500">{t('workbench.common.loading', 'Loading…')}</p>
        <WorkbenchChatPanel />
      </>
    );
  if (!bids || bids.length === 0)
    return (
      <>
        <EmptyRoom workspaceId={workspaceId} />
        <WorkbenchChatPanel />
      </>
    );

  const now = new Date();
  const grouped: Record<Band, Bid[]> = { needsYouNow: [], active: [], submitted: [], closed: [] };
  for (const bid of bids) grouped[bandOf(bid, now)].push(bid);

  return (
    <>
      <div className="flex flex-col gap-8">
        {BAND_ORDER.filter((b) => grouped[b].length > 0).map((band) => (
          <section key={band} className="flex flex-col gap-3">
            <h2
              className={cn(
                'text-sm font-semibold',
                band === 'needsYouNow' ? 'text-brand-700' : 'text-ink-700',
              )}
            >
              {t(`bid.bands.${band}`)}
            </h2>
            <ul className="flex flex-col gap-2">
              {grouped[band].map((bid) => (
                <li key={bid.id}>
                  <BidRow bid={bid} workspaceId={workspaceId} workbenchId={workbench.id} />
                </li>
              ))}
            </ul>
          </section>
        ))}
      </div>
      <WorkbenchChatPanel />
    </>
  );
}

function BidRow({
  bid,
  workspaceId,
  workbenchId,
}: {
  bid: Bid;
  workspaceId: string;
  workbenchId: string;
}) {
  const { t } = useTranslation();
  const deadline = bid.tenderAvailable ? deadlineInfo(bid.tenderDeadline, new Date()) : null;
  const tier = bid.fitTier ? (bid.fitTier as FitTier) : null;
  return (
    <Card padding="none">
      <Link
        to="/workspaces/$workspaceId/workbench/$workbenchId/bids/$bidId"
        params={{ workspaceId, workbenchId, bidId: bid.id }}
        className="flex items-center gap-4 rounded-2xl p-4 no-underline outline-none transition-colors duration-150 hover:bg-cream-50 focus-visible:ring-2 focus-visible:ring-brand-600"
      >
        <span className="min-w-0 flex-1">
          <span className="block truncate font-medium text-ink-900">
            {bid.tenderAvailable
              ? bid.tenderTitle
              : t('bid.tombstone.title', 'This tender is no longer available')}
          </span>
          {bid.tenderAvailable && bid.tenderBuyerName && (
            <span className="block truncate text-xs text-ink-500">{bid.tenderBuyerName}</span>
          )}
          <span className="mt-1 block text-xs text-ink-400">
            {t(`bid.stage.${bid.stage}`)} ·{' '}
            {t('bid.checklist.progress', { done: bid.checklistDone, total: bid.checklistTotal })}
          </span>
        </span>
        {tier && (
          <Pill tone={fitTierPillTone(tier)} className={cn('shrink-0', fitTierPillClassName(tier))}>
            {t(`tenders.fit.tier.${tier === 'long_shot' ? 'longShot' : tier}`)}
          </Pill>
        )}
        {deadline && (
          <Pill tone={deadline.tone} className="shrink-0">
            {deadline.days < 0
              ? t('tenders.deadline.expired')
              : deadline.days === 0
                ? t('tenders.deadline.today')
                : t('tenders.deadline.days', { count: deadline.days })}
          </Pill>
        )}
      </Link>
    </Card>
  );
}

function EmptyRoom({ workspaceId }: { workspaceId: string }) {
  const { t } = useTranslation();
  const link = useTenderLink();
  const { results, needsProfile } = useClientShortlist(workspaceId);

  if (needsProfile) {
    return (
      <EmptyState
        title={t('bid.empty.needsProfileTitle', 'Set up a client profile to see fit')}
        description={t(
          'bid.empty.description',
          'Take a tender into this workbench to start preparing it.',
        )}
        action={
          <Link to="/tenders" className="text-brand-700 underline">
            {t('bid.empty.needsProfileCta', 'Set up client profile')}
          </Link>
        }
      />
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <EmptyState
        title={t('bid.empty.title', 'No bandi tracked yet')}
        description={t(
          'bid.empty.description',
          'Take a tender into this workbench to start preparing it.',
        )}
      />
      {results.length > 0 && (
        <section className="flex flex-col gap-3">
          <h2 className="text-sm font-semibold text-ink-700">
            {t('bid.empty.seedTitle', 'Best-fit bandi to prepare')}
          </h2>
          <ul className="flex flex-col gap-2">
            {results.map(
              (r) =>
                r.tender && (
                  <li key={r.tender.id}>
                    {link(
                      r.tender.id,
                      <TenderResultCard
                        tender={r.tender}
                        fitTier={r.fitTier as FitTier}
                        reason={r.reason}
                      />,
                      'block no-underline',
                    )}
                  </li>
                ),
            )}
          </ul>
        </section>
      )}
    </div>
  );
}
