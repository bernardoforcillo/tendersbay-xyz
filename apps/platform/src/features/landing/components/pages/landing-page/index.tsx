import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { LandingTemplate } from '~/features/landing/components/templates';

export function LandingPage() {
  const { t } = useTranslation();

  useEffect(() => {
    document.title = t('landing.meta.title');
  }, [t]);

  return <LandingTemplate />;
}
