import { Card, cn, Pill } from '@tendersbay/components/core';
import type { TenderResult } from '@tendersbay/proto/tender/v1/tender_pb';
import { useTranslation } from 'react-i18next';
import { deadlineInfo, formatTenderValue } from '../tender-feed';

export type TenderResultCardProps = {
  tender: TenderResult;
  className?: string;
};

/**
 * Presentational result card for a single tender in a search feed. No
 * fetching, no link/onPress (there's no tender detail page yet) and no
 * match-% / reason line (doesn't exist yet) — purely renders the fields the
 * search RPC already returns.
 */
export function TenderResultCard({ tender, className }: TenderResultCardProps) {
  const { t, i18n } = useTranslation();

  const metaTag = [tender.cpv, tender.country].filter(Boolean).join(' · ');
  const deadline = deadlineInfo(tender.deadline, new Date());
  const deadlineLabel = deadline
    ? deadline.days < 0
      ? t('tenders.deadline.expired', { defaultValue: 'Closed' })
      : deadline.days === 0
        ? t('tenders.deadline.today', { defaultValue: 'Closes today' })
        : t('tenders.deadline.days', {
            count: deadline.days,
            defaultValue: '{{count}} days left',
          })
    : null;

  const value = formatTenderValue(tender.value, tender.currency, i18n.language);
  const metaLine = `${value ?? t('tenders.value.unknown', { defaultValue: 'Value undisclosed' })} · ${t(
    `tenders.status.${tender.status}`,
    { defaultValue: tender.status },
  )}`;

  return (
    <Card padding="none" className={cn('p-4', className)}>
      <div className="flex items-start justify-between gap-2">
        {metaTag && (
          <p className="font-mono text-[10px] uppercase tracking-wide text-ink-400">{metaTag}</p>
        )}
        {deadline && (
          <Pill tone={deadline.tone} className="ml-auto shrink-0">
            {deadlineLabel}
          </Pill>
        )}
      </div>
      <p className="mt-1 truncate text-sm font-medium text-ink-900">{tender.title}</p>
      {tender.buyerName && <p className="truncate text-xs text-ink-500">{tender.buyerName}</p>}
      <p className="mt-1 text-xs text-ink-500">{metaLine}</p>
    </Card>
  );
}
