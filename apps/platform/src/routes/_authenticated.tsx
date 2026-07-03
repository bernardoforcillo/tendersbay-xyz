import { createFileRoute, Outlet, redirect } from '@tanstack/react-router';
import { detectLocale } from '~/i18n/detect-locale';

export const Route = createFileRoute('/_authenticated')({
  beforeLoad: ({ context }) => {
    if (!context.auth.isAuthenticated) {
      throw redirect({
        to: '/$locale/auth/login',
        params: { locale: detectLocale() },
      });
    }
  },
  component: () => <Outlet />,
});
