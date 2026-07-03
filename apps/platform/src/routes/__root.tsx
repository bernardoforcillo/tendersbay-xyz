import { createRootRouteWithContext, Outlet } from '@tanstack/react-router';
import { LayoutGroup } from 'motion/react';
import type { AuthStore } from '~/store/auth';

export const Route = createRootRouteWithContext<{ auth: AuthStore }>()({
  component: RootLayout,
});

function RootLayout() {
  // LayoutGroup persists motion's shared-layout context across route swaps, so a
  // `layoutId` element (the account search dock) animates between pages.
  return (
    <LayoutGroup>
      <Outlet />
    </LayoutGroup>
  );
}
