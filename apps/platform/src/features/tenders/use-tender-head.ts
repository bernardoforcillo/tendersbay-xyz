import type { TenderDetail } from '@tendersbay/proto/tender/v1/tender_pb';
import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';

/** Sets document.title + description meta while a tender detail is mounted (client-side; crawlers get the server-injected tags). */
export function useTenderHead(tender: TenderDetail | null): void {
  const { t } = useTranslation();
  useEffect(() => {
    if (!tender) return;
    const prevTitle = document.title;
    document.title = t('tenders.detail.meta.title', { title: tender.title });
    const meta = document.querySelector('meta[name="description"]');
    const prevDesc = meta?.getAttribute('content') ?? null;
    meta?.setAttribute('content', t('tenders.detail.meta.description', { title: tender.title }));
    return () => {
      document.title = prevTitle;
      if (meta && prevDesc !== null) meta.setAttribute('content', prevDesc);
    };
  }, [tender, t]);
}
