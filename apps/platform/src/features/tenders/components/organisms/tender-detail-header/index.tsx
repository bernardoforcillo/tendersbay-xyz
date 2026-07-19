import { cn, Pill } from '@tendersbay/components/core';
import type { TenderDetail } from '@tendersbay/proto/tender/v1/tender_pb';
import { useTranslation } from 'react-i18next';
import {
  countryFlag,
  countryName,
  deadlineInfo,
} from '~/features/account/components/organisms/tender-feed';

export function TenderDetailHeader({ tender }: { tender: TenderDetail }) {
  const { t, i18n } = useTranslation();
  const Flag = tender.country ? countryFlag(tender.country) : null;
  const originName = tender.country ? countryName(tender.country, i18n.language) : null;
  const deadline = deadlineInfo(tender.deadline, new Date());
  const deadlineLabel = deadline
    ? deadline.days < 0
      ? t('tenders.deadline.expired')
      : deadline.days === 0
        ? t('tenders.deadline.today')
        : t('tenders.deadline.days', { count: deadline.days })
    : null;
  return (
    <header className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        {Flag && (
          <span
            title={originName ?? undefined}
            className="block w-6 shrink-0 overflow-hidden rounded ring-1 ring-ink-900/10"
          >
            <Flag aria-label={originName ?? undefined} className="block h-auto w-full" />
          </span>
        )}
        {tender.source && (
          <span className="inline-flex items-center rounded-md border border-cream-300 bg-cream-100 px-1.5 py-0.5 font-mono text-[10px] font-semibold uppercase tracking-wide text-ink-500">
            {tender.source.toUpperCase()}
          </span>
        )}
        {deadline && (
          <Pill tone={deadline.tone} className={cn('ml-auto shrink-0')}>
            {deadlineLabel}
          </Pill>
        )}
      </div>
      <h1 className="text-2xl font-semibold leading-snug text-ink-900 sm:text-3xl">
        {tender.title}
      </h1>
      {tender.buyerName && <p className="text-sm text-ink-500">{tender.buyerName}</p>}
    </header>
  );
}
