import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { getAnalytics } from '../../posthog';

/** Registers the active locale as a PostHog super-property so events segment by `/<locale>/`. */
export function useLocaleProperty(): void {
  const { i18n } = useTranslation();
  useEffect(() => {
    const posthog = getAnalytics();
    if (!posthog) {
      return;
    }
    const register = (locale: string) => posthog.register({ locale });
    register(i18n.language);
    i18n.on('languageChanged', register);
    return () => {
      i18n.off('languageChanged', register);
    };
  }, [i18n]);
}
