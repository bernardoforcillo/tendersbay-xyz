import { createFileRoute } from '@tanstack/react-router';
import { AccountExplorePage } from '~/features/account';

export const Route = createFileRoute('/_authenticated/explore/')({
  component: AccountExplorePage,
});
