import type { TenderDetail } from '@tendersbay/proto/tender/v1/tender_pb';
import { useTranslation } from 'react-i18next';

export function TenderLots({ tender }: { tender: TenderDetail }) {
  const { t } = useTranslation();
  if (tender.lots.length === 0) return null;
  return (
    <section className="space-y-2">
      <h2 className="text-sm font-semibold text-ink-900">{t('tenders.detail.lots')}</h2>
      <ul className="divide-y divide-cream-200">
        {tender.lots.map((lot) => (
          <li key={lot.ref} className="py-2">
            <p className="text-sm text-ink-900">{lot.title || lot.ref}</p>
            {lot.cpv && (
              <p className="font-mono text-[10px] uppercase tracking-wide text-ink-400">
                {lot.cpv}
              </p>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
}
