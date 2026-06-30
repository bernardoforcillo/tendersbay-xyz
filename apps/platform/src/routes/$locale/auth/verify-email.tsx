import { createFileRoute } from '@tanstack/react-router'
import { VerifyEmailPage } from '~/features/auth'

export const Route = createFileRoute('/$locale/auth/verify-email')({
  validateSearch: (search: Record<string, unknown>) => ({
    token: search['token'] as string | undefined,
    type: search['type'] as string | undefined,
  }),
  component: VerifyEmailPage,
})
