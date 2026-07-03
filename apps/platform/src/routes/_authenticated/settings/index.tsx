import { createFileRoute } from '@tanstack/react-router';
import { AccountSettingsPage } from '~/features/account';

export const Route = createFileRoute('/_authenticated/settings/')({
  component: AccountSettingsPage,
});
