import { cn } from '@tendersbay/components/core';
import type { TenderDetail } from '@tendersbay/proto/tender/v1/tender_pb';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import {
  countryName,
  formatTenderValue,
} from '~/features/account/components/organisms/tender-feed';

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-0.5 border-t border-cream-200 py-2">
      <dt className="font-mono text-[10px] font-semibold uppercase tracking-wide text-ink-400">
        {label}
      </dt>
      <dd className="text-sm text-ink-900">{children}</dd>
    </div>
  );
}

export function TenderDetailMeta({ tender }: { tender: TenderDetail }) {
  const { t, i18n } = useTranslation();
  const value = formatTenderValue(tender.value, tender.currency, i18n.language);
  return (
    <dl className={cn('grid grid-cols-1 gap-x-8 sm:grid-cols-2')}>
      <Row label={t('tenders.detail.value')}>
        <span className="font-mono font-medium tabular-nums text-brand-700">
          {value ?? t('tenders.value.unknown')}
        </span>
      </Row>
      <Row label={t('tenders.detail.status')}>
        {t(`tenders.status.${tender.status}`, { defaultValue: tender.status })}
      </Row>
      {tender.procedureType && (
        <Row label={t('tenders.detail.procedure')}>{tender.procedureType}</Row>
      )}
      {tender.cpv && <Row label={t('tenders.detail.cpv')}>{tender.cpv}</Row>}
      {tender.cpvSecondary.length > 0 && (
        <Row label={t('tenders.detail.cpvSecondary')}>{tender.cpvSecondary.join(', ')}</Row>
      )}
      {tender.country && (
        <Row label={t('tenders.detail.region')}>
          {countryName(tender.country, i18n.language)}
          {tender.nuts ? ` · ${tender.nuts}` : ''}
        </Row>
      )}
      {tender.language && (
        <Row label={t('tenders.detail.language')}>{tender.language.toUpperCase()}</Row>
      )}
      {tender.buyerName && <Row label={t('tenders.detail.buyer')}>{tender.buyerName}</Row>}
    </dl>
  );
}
