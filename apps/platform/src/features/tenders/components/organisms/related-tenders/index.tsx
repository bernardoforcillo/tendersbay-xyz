import type { TenderResult } from '@tendersbay/proto/tender/v1/tender_pb';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

/** Route-agnostic: the caller supplies renderItem (the linked card), so this
 *  organism never references a route and compiles standalone. */
export function RelatedTenders({
  related,
  renderItem,
}: {
  related: TenderResult[];
  renderItem: (t: TenderResult) => ReactNode;
}) {
  const { t } = useTranslation();
  return (
    <section className="space-y-3">
      <h2 className="text-sm font-semibold text-ink-900">{t('tenders.detail.related')}</h2>
      {related.length === 0 ? (
        <p className="text-sm text-ink-500">{t('tenders.detail.relatedNone')}</p>
      ) : (
        <div className="space-y-3">
          {related.map((tr) => (
            <div key={tr.id}>{renderItem(tr)}</div>
          ))}
        </div>
      )}
    </section>
  );
}
