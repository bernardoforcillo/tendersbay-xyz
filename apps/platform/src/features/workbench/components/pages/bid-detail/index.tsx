import { Link, useParams } from '@tanstack/react-router';
import { Banner, Button, Card, cn, Field, Pill, Select } from '@tendersbay/components/core';
import type { ChecklistItem } from '@tendersbay/proto/bid/v1/bid_pb';
import { usePostHog } from 'posthog-js/react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  deadlineInfo,
  type FitTier,
  fitReasonFragments,
  fitTierPillClassName,
  fitTierPillTone,
} from '~/features/account/components/organisms/tender-feed';
import { useWorkbenchContext } from '~/features/workbench/context';
import { useBid, useChecklist } from '~/features/workbench/hooks';
import { can, Permission } from '~/features/workbench/permissions';
import { bidClient } from '~/lib/api/client';

const STAGES = ['shortlisted', 'preparing', 'submitted'] as const;
const FIT_TIER_KEY: Record<FitTier, string> = {
  strong: 'tenders.fit.tier.strong',
  possible: 'tenders.fit.tier.possible',
  long_shot: 'tenders.fit.tier.longShot',
};

export function BidDetailPage() {
  const { t } = useTranslation();
  const { bidId, workspaceId, workbenchId } = useParams({
    from: '/_authenticated/workspaces/$workspaceId/workbench/$workbenchId/bids/$bidId',
  });
  const { myPermissions } = useWorkbenchContext();
  const canManage = can(myPermissions, Permission.ManageWorkbench);
  const posthog = usePostHog();

  const { data: bid, loading, error, refetch: refetchBid } = useBid(bidId);
  const { data: items, refetch: refetchChecklist } = useChecklist(bidId);

  async function decide(d: 'go' | 'no_go') {
    await bidClient.setGoNoGo({ workbenchId, bidId, goNoGo: d });
    posthog?.capture('bid_decision_recorded', { location: 'bid_detail', decision: d });
    refetchBid();
  }
  async function advance(fromStage: string) {
    await bidClient.advanceStage({ workbenchId, bidId });
    posthog?.capture('bid_stage_changed', { location: 'bid_detail', from_stage: fromStage });
    refetchBid();
  }
  async function closeOutcome(o: 'won' | 'lost' | 'withdrawn') {
    await bidClient.recordOutcome({ workbenchId, bidId, outcome: o });
    posthog?.capture('bid_outcome_recorded', { location: 'bid_detail', outcome: o });
    refetchBid();
  }
  async function saveItem(item: ChecklistItem, status: string, note: string) {
    await bidClient.upsertChecklistAnswer({
      workbenchId,
      bidId,
      itemCode: item.itemCode,
      status,
      note,
    });
    if (status === 'done') {
      posthog?.capture('checklist_item_completed', {
        location: 'bid_detail',
        section_code: item.sectionCode,
        required: item.required,
      });
    }
    refetchChecklist();
    refetchBid();
  }

  if (loading)
    return <p className="text-sm text-ink-500">{t('workbench.common.loading', 'Loading…')}</p>;
  if (error || !bid)
    return (
      <Banner tone="error">
        {error ?? t('bid.errors.generic', 'Something went wrong. Please try again.')}
      </Banner>
    );

  const deadline = bid.tenderAvailable ? deadlineInfo(bid.tenderDeadline, new Date()) : null;
  const tier = bid.fitTier ? (bid.fitTier as FitTier) : null;
  const stageIndex = STAGES.indexOf(bid.stage as (typeof STAGES)[number]);
  const isNoGo = bid.goNoGo === 'no_go';
  const grouped: Record<string, ChecklistItem[]> = {};
  for (const it of items ?? []) {
    const list = grouped[it.sectionCode] ?? [];
    list.push(it);
    grouped[it.sectionCode] = list;
  }
  const sectionOrder = [...new Set((items ?? []).map((i) => i.sectionCode))];

  return (
    <div className="mx-auto flex w-full max-w-2xl flex-col gap-6">
      <Link
        to="/workspaces/$workspaceId/workbench/$workbenchId"
        params={{ workspaceId, workbenchId }}
        className="text-sm text-brand-700 no-underline hover:underline"
      >
        {t('bid.detail.backToPortfolio', 'Back to portfolio')}
      </Link>

      {bid.tenderAvailable ? (
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
      ) : (
        <Card className="border-cream-300 bg-cream-50">
          <h1 className="text-lg font-semibold text-ink-900">{t('bid.tombstone.title')}</h1>
          <p className="mt-1 text-sm text-ink-500">
            {t(
              'bid.tombstone.description',
              'The source portal removed this listing. Your checklist and notes are preserved.',
            )}
          </p>
        </Card>
      )}

      <Card>
        <h2 className="text-sm font-semibold text-ink-700">
          {t('bid.detail.goNoGoTitle', 'Decision')}
        </h2>
        <div className="mt-3 flex flex-wrap items-center gap-2">
          <Pill
            tone={bid.goNoGo === 'go' ? 'match' : 'neutral'}
            className={cn(isNoGo && 'grayscale')}
          >
            {t(`bid.goNoGo.${bid.goNoGo === 'no_go' ? 'noGo' : bid.goNoGo}`)}
          </Pill>
          {canManage && (
            <>
              <Button variant="ghost" onPress={() => void decide('go')}>
                {t('bid.actions.markGo')}
              </Button>
              <Button variant="ghost" onPress={() => void decide('no_go')}>
                {t('bid.actions.markNoGo', 'No-go')}
              </Button>
            </>
          )}
        </div>
      </Card>

      <Card>
        <h2 className="text-sm font-semibold text-ink-700">
          {t('bid.detail.stageTitle', 'Stage')}
        </h2>
        <ol className="mt-3 flex items-center gap-2">
          {STAGES.map((s, i) => (
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
        {canManage && bid.stage !== 'submitted' && (
          <Button className="mt-3" isDisabled={isNoGo} onPress={() => void advance(bid.stage)}>
            {t('bid.actions.advance', 'Advance stage')}
          </Button>
        )}
      </Card>

      <Card>
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-semibold text-ink-700">
            {t('bid.checklist.title', 'ESPD / DGUE checklist')}
          </h2>
          <span className="text-xs text-ink-500">
            {t('bid.checklist.progress', { done: bid.checklistDone, total: bid.checklistTotal })}
          </span>
        </div>
        <div className="mt-3 flex flex-col gap-4">
          {sectionOrder.map((sec) => (
            <div key={sec}>
              <p className="text-xs font-semibold uppercase tracking-wide text-ink-400">
                {t(`bid.checklist.sections.${sec}`, sec)}
              </p>
              <div className="mt-2 flex flex-col gap-2">
                {(grouped[sec] ?? []).map((item) => (
                  <ChecklistRow key={item.id} item={item} canManage={canManage} onSave={saveItem} />
                ))}
              </div>
            </div>
          ))}
        </div>
      </Card>

      {bid.goNoGo === 'go' && (
        <Card>
          <h2 className="text-sm font-semibold text-ink-700">
            {t('bid.detail.outcomeTitle', 'Outcome')}
          </h2>
          <div className="mt-3 flex flex-wrap items-center gap-2">
            {bid.outcome ? (
              <Pill
                tone={bid.outcome === 'won' ? 'match' : 'neutral'}
                className={cn(bid.outcome !== 'won' && 'grayscale')}
              >
                {t(`bid.outcome.${bid.outcome}`)}
              </Pill>
            ) : (
              canManage && (
                <>
                  <Button variant="ghost" onPress={() => void closeOutcome('won')}>
                    {t('bid.outcome.won', 'Won')}
                  </Button>
                  <Button variant="ghost" onPress={() => void closeOutcome('lost')}>
                    {t('bid.outcome.lost', 'Lost')}
                  </Button>
                  <Button variant="ghost" onPress={() => void closeOutcome('withdrawn')}>
                    {t('bid.outcome.withdrawn', 'Withdrawn')}
                  </Button>
                </>
              )
            )}
          </div>
        </Card>
      )}
    </div>
  );
}

function ChecklistRow({
  item,
  canManage,
  onSave,
}: {
  item: ChecklistItem;
  canManage: boolean;
  onSave: (item: ChecklistItem, status: string, note: string) => void;
}) {
  const { t } = useTranslation();
  const [note, setNote] = useState(item.note);
  return (
    <div className="flex flex-col gap-2 rounded-xl border border-cream-200 p-3 sm:flex-row sm:items-end">
      <div className="flex-1">
        <p className="text-sm text-ink-900">
          {t(`bid.checklist.items.${item.itemCode}`, item.itemCode)}
        </p>
        {item.required && (
          <span className="text-[10px] font-semibold uppercase tracking-wide text-ink-400">
            {t('bid.checklist.required', 'Required')}
          </span>
        )}
      </div>
      <Select
        label={t('bid.checklist.statusLabel', 'Status')}
        className="w-32"
        value={item.status}
        disabled={!canManage}
        onChange={(e) => onSave(item, e.target.value, note)}
      >
        <option value="pending">{t('bid.checklist.status.pending', 'Pending')}</option>
        <option value="done">{t('bid.checklist.status.done', 'Done')}</option>
        <option value="na">{t('bid.checklist.status.na', 'N/A')}</option>
      </Select>
      <Field
        label={t('bid.checklist.noteLabel', 'Note')}
        placeholder={t('bid.checklist.notePlaceholder', 'Add a note…')}
        className="flex-1"
        value={note}
        onChange={setNote}
        isDisabled={!canManage}
        onBlur={() => {
          if (note !== item.note) onSave(item, item.status, note);
        }}
      />
    </div>
  );
}
