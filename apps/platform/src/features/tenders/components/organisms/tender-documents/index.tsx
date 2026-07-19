import type { TenderDetail } from '@tendersbay/proto/tender/v1/tender_pb';
import { useTranslation } from 'react-i18next';

export function TenderDocuments({ tender }: { tender: TenderDetail }) {
  const { t } = useTranslation();
  const links = tender.documents.map((d) => ({ url: d.url, type: d.type }));
  if (tender.sourceUrl) links.push({ url: tender.sourceUrl, type: 'source' });
  if (links.length === 0) return null;
  return (
    <section className="space-y-2">
      <h2 className="text-sm font-semibold text-ink-900">{t('tenders.detail.documents')}</h2>
      <ul className="space-y-1.5">
        {links.map((d) => (
          <li key={d.url}>
            <a
              href={d.url}
              target="_blank"
              rel="noopener noreferrer nofollow"
              className="text-sm text-brand-700 underline underline-offset-2"
            >
              {d.type === 'notice'
                ? t('tenders.detail.officialNotice')
                : t('tenders.detail.viewSource')}
            </a>
          </li>
        ))}
      </ul>
    </section>
  );
}
