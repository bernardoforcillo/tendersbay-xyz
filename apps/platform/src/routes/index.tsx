import { createFileRoute, redirect } from '@tanstack/react-router';
import { AccountOverviewPage } from '~/features/account';
import { detectLocale } from '~/i18n/detect-locale';

export const Route = createFileRoute('/')({
  beforeLoad: ({ context }) => {
    if (!context.auth.isAuthenticated) {
      throw redirect({ to: '/$locale', params: { locale: detectLocale() } });
    }
  },
  component: AccountOverviewPage,
});
