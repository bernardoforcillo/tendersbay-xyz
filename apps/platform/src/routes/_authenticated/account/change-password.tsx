import { createFileRoute } from '@tanstack/react-router'
import { ChangePasswordPage } from '~/features/account'

export const Route = createFileRoute('/_authenticated/account/change-password')({
  component: ChangePasswordPage,
})
