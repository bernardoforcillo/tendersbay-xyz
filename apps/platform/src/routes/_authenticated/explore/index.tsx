import { createFileRoute } from '@tanstack/react-router';
import { AccountExplorePage } from '~/features/account';

export const Route = createFileRoute('/_authenticated/explore/')({
  validateSearch: (search: Record<string, unknown>): { q?: string } => ({
    q: typeof search.q === 'string' ? search.q : undefined,
  }),
  component: AccountExplorePage,
});
