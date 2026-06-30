import { createFileRoute } from '@tanstack/react-router'
import { ResetPasswordPage } from '~/features/auth'

export const Route = createFileRoute('/$locale/auth/reset-password')({
  validateSearch: (search: Record<string, unknown>) => ({
    token: search['token'] as string | undefined,
  }),
  component: ResetPasswordPage,
})
