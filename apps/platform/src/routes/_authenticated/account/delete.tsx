import { createFileRoute } from '@tanstack/react-router'
import { DeleteAccountPage } from '~/features/account'

export const Route = createFileRoute('/_authenticated/account/delete')({
  component: DeleteAccountPage,
})
