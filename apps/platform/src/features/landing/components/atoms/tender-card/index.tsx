import { cn } from '@tendersbay/components/core';
import { useTranslation } from 'react-i18next';

export type Tender = {
  id: string;
  entity: string;
  object: string;
  value: string;
  deadlineDays: number;
  scoutCount: number;
};

export function TenderCard({ tender, className }: { tender: Tender; className?: string }) {
  const { t } = useTranslation();
  return (
    <div
      className={cn(
        'w-52 rounded-2xl bg-ink-900 p-4 text-ink-100 shadow-2xl shadow-ink-900/30',
        className,
      )}
    >
      <p className="font-mono text-[10px] font-semibold uppercase tracking-[0.12em] text-brand-300">
        {tender.entity}
      </p>
      <p className="mt-1.5 mb-3 text-sm font-bold leading-tight text-white">{tender.object}</p>
      <dl className="text-xs">
        <div className="flex justify-between border-t border-white/10 py-1.5">
          <dt className="font-mono text-[11px] uppercase tracking-wide text-ink-300">
            {t('landing.tenderCard.value')}
          </dt>
          <dd className="font-mono font-medium tabular-nums text-brand-300">{tender.value}</dd>
        </div>
        <div className="flex justify-between border-t border-white/10 py-1.5">
          <dt className="font-mono text-[11px] uppercase tracking-wide text-ink-300">
            {t('landing.tenderCard.deadline')}
          </dt>
          <dd className="font-mono font-medium tabular-nums text-brand-300">
            {t('landing.tenderCard.daysLeft', { count: tender.deadlineDays })}
          </dd>
        </div>
      </dl>
    </div>
  );
}
