import { createFileRoute, redirect } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/settings/account')({
  beforeLoad: () => {
    throw redirect({ to: '/settings' });
  },
});
