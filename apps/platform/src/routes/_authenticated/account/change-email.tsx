import { createFileRoute } from '@tanstack/react-router'
import { ChangeEmailPage } from '~/features/account'

export const Route = createFileRoute('/_authenticated/account/change-email')({
  component: ChangeEmailPage,
})
