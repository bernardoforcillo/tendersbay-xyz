import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { LandingTemplate } from '~/features/landing/components/templates';

/** Update an existing head meta tag's content; never creates duplicates. */
function setMeta(selector: string, content: string) {
  document.head.querySelector(selector)?.setAttribute('content', content);
}

export function LandingPage() {
  const { t } = useTranslation();

  useEffect(() => {
    const title = t('landing.meta.title');
    const description = t('landing.meta.description');
    document.title = title;
    setMeta('meta[name="description"]', description);
    setMeta('meta[property="og:title"]', title);
    setMeta('meta[property="og:description"]', description);
    setMeta('meta[name="twitter:title"]', title);
    setMeta('meta[name="twitter:description"]', description);
  }, [t]);

  return <LandingTemplate />;
}
