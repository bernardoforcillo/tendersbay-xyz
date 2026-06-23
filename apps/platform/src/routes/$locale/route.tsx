import { createFileRoute, Outlet, redirect } from '@tanstack/react-router';
import { i18n } from '~/i18n';
import { DEFAULT_LOCALE, isSupportedLocale } from '~/i18n/detect-locale';

export const Route = createFileRoute('/$locale')({
  beforeLoad: ({ params }) => {
    if (!isSupportedLocale(params.locale)) {
      throw redirect({ to: '/$locale', params: { locale: DEFAULT_LOCALE } });
    }
    void i18n.changeLanguage(params.locale);
    document.documentElement.lang = params.locale;
  },
  component: LocaleLayout,
});

function LocaleLayout() {
  return <Outlet />;
}
