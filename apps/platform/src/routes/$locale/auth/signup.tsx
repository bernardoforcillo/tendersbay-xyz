import { createFileRoute } from '@tanstack/react-router';
import { SignupPage } from '~/features/auth';

export const Route = createFileRoute('/$locale/auth/signup')({
  validateSearch: (search: Record<string, unknown>): { redirect?: string } =>
    typeof search.redirect === 'string' ? { redirect: search.redirect } : {},
  component: SignupPage,
});
