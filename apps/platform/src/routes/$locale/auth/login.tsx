import { createFileRoute } from '@tanstack/react-router';
import { LoginPage } from '~/features/auth';

export const Route = createFileRoute('/$locale/auth/login')({
  validateSearch: (search: Record<string, unknown>): { redirect?: string } =>
    typeof search.redirect === 'string' ? { redirect: search.redirect } : {},
  component: LoginPage,
});
