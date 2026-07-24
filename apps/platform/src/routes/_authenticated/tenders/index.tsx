import { createFileRoute } from '@tanstack/react-router';
import { AccountTendersPage } from '~/features/account';

export const Route = createFileRoute('/_authenticated/tenders/')({
  validateSearch: (search: Record<string, unknown>): { q?: string } => ({
    q: typeof search.q === 'string' ? search.q : undefined,
  }),
  component: AccountTendersPage,
});
