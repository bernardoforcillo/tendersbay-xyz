import { createFileRoute, redirect } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/settings/change-password')({
  beforeLoad: () => {
    throw redirect({ to: '/settings/account' });
  },
});
