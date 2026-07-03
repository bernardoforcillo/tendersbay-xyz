import { createFileRoute, redirect } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/settings/change-email')({
  beforeLoad: () => {
    throw redirect({ to: '/settings/account' });
  },
});
