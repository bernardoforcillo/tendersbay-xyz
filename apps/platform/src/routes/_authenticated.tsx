import { createFileRoute, Outlet, redirect } from '@tanstack/react-router';
import { ToastContainer } from '~/features/toast';
import { detectLocale } from '~/i18n/detect-locale';

export const Route = createFileRoute('/_authenticated')({
  beforeLoad: ({ context, location }) => {
    if (!context.auth.isAuthenticated) {
      throw redirect({
        to: '/$locale/auth/login',
        params: { locale: detectLocale() },
        // Preserve the intended destination so login can bounce back to it.
        search: { redirect: location.href },
      });
    }
  },
  component: AuthenticatedLayout,
});

function AuthenticatedLayout() {
  return (
    <>
      <Outlet />
      <ToastContainer />
    </>
  );
}
