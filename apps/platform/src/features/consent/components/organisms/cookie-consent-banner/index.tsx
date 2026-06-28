import { AnimatePresence, motion } from 'motion/react';
import { Button } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { getAnalytics } from '~/analytics';
import { useConsent } from '~/features/consent/hooks/use-consent';

export function CookieConsentBanner() {
  const { t } = useTranslation();
  const { status, grant, deny } = useConsent();
  const visible = status === null && getAnalytics() !== null;

  return (
    <AnimatePresence>
      {visible ? (
        <motion.div
          role="dialog"
          aria-label={t('consent.title')}
          initial={{ opacity: 0, y: 24 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: 24 }}
          transition={{ duration: 0.2 }}
          className="fixed inset-x-4 bottom-4 z-50 mx-auto max-w-xl rounded-2xl border border-ink-200 bg-cream-50 p-5 shadow-soft-lg sm:flex sm:items-center sm:gap-5"
        >
          <div className="flex-1">
            <p className="font-semibold text-ink-900">{t('consent.title')}</p>
            <p className="mt-1 text-ink-700 text-sm leading-relaxed">{t('consent.body')}</p>
          </div>
          <div className="mt-4 flex shrink-0 gap-2 sm:mt-0">
            <Button
              onPress={deny}
              className="rounded-xl px-4 py-2 text-ink-700 text-sm transition-colors hover:bg-ink-100"
            >
              {t('consent.reject')}
            </Button>
            <Button
              onPress={grant}
              className="rounded-xl bg-ink-900 px-4 py-2 font-medium text-cream-50 text-sm transition-colors hover:bg-ink-800"
            >
              {t('consent.accept')}
            </Button>
          </div>
        </motion.div>
      ) : null}
    </AnimatePresence>
  );
}
