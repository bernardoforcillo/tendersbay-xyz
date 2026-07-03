import { createFileRoute } from '@tanstack/react-router';
import { SignupPage } from '~/features/auth';

export const Route = createFileRoute('/$locale/auth/signup')({
  component: SignupPage,
});
