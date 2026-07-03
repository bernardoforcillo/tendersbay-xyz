import { createFileRoute } from '@tanstack/react-router';
import { ForgotPasswordPage } from '~/features/auth';

export const Route = createFileRoute('/$locale/auth/forgot-password')({
  component: ForgotPasswordPage,
});
