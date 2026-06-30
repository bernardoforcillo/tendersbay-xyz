import { createRootRouteWithContext, Outlet } from '@tanstack/react-router';
import type { AuthStore } from '~/store/auth';

export const Route = createRootRouteWithContext<{ auth: AuthStore }>()({
  component: RootLayout,
});

function RootLayout() {
  return <Outlet />;
}
