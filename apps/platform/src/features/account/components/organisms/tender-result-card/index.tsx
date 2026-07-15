import { Card, cn, Pill } from '@tendersbay/components/core';
import type { TenderResult } from '@tendersbay/proto/tender/v1/tender_pb';
import { useTranslation } from 'react-i18next';
import {
  countryFlag,
  countryName,
  deadlineInfo,
  formatTenderValue,
  tenderTitle,
} from '../tender-feed';

export type TenderResultCardProps = {
  tender: TenderResult;
  className?: string;
};

/**
 * Presentational result card for a single tender in a search feed. No
 * fetching, no link/onPress (there's no tender detail page yet) and no
 * match-% / reason line (doesn't exist yet) — purely renders the fields the
 * search RPC already returns.
 *
 * The header is a provenance rail: a country flag (the tender's origin) and a
 * source stamp (the portal it was published on, e.g. TED), with the deadline
 * pill trailing. Title leads the body; value, status and CPV close the card.
 */
export function TenderResultCard({ tender, className }: TenderResultCardProps) {
  const { t, i18n } = useTranslation();

  const Flag = tender.country ? countryFlag(tender.country) : null;
  const originName = tender.country ? countryName(tender.country, i18n.language) : null;

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
  const statusLabel = t(`tenders.status.${tender.status}`, { defaultValue: tender.status });

  return (
    <Card
      padding="none"
      className={cn('p-4 transition-shadow duration-200 hover:shadow-soft-md', className)}
    >
      <div className="flex items-center gap-2">
        {Flag ? (
          <span
            title={originName ?? undefined}
            className="block w-6 shrink-0 overflow-hidden rounded ring-1 ring-ink-900/10"
          >
            <Flag aria-label={originName ?? undefined} className="block h-auto w-full" />
          </span>
        ) : (
          originName && (
            <span className="font-mono text-[10px] font-semibold uppercase tracking-wide text-ink-400">
              {originName}
            </span>
          )
        )}
        {tender.source && (
          <span className="inline-flex items-center rounded-md border border-cream-300 bg-cream-100 px-1.5 py-0.5 font-mono text-[10px] font-semibold uppercase tracking-wide text-ink-500">
            {tender.source.toUpperCase()}
          </span>
        )}
        {deadline && (
          <Pill tone={deadline.tone} className="ml-auto shrink-0">
            {deadlineLabel}
          </Pill>
        )}
      </div>

      <p className="mt-2.5 line-clamp-2 text-sm font-medium leading-snug text-ink-900">
        {tenderTitle(tender.title, tender.country)}
      </p>
      {tender.buyerName && (
        <p className="mt-0.5 truncate text-xs text-ink-500">{tender.buyerName}</p>
      )}

      <div className="mt-3 flex items-baseline justify-between gap-3 border-t border-cream-200 pt-2.5">
        <p className="min-w-0 truncate text-xs text-ink-500">
          <span className="font-mono font-medium tabular-nums text-brand-700">
            {value ?? t('tenders.value.unknown', { defaultValue: 'Value undisclosed' })}
          </span>
          <span className="mx-1.5 text-cream-400">·</span>
          <span>{statusLabel}</span>
        </p>
        {tender.cpv && (
          <span className="shrink-0 font-mono text-[10px] uppercase tracking-wide text-ink-400">
            {tender.cpv}
          </span>
        )}
      </div>
    </Card>
  );
}
