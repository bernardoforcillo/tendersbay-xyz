import { createFileRoute, Link, useNavigate } from '@tanstack/react-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { PageHeader, SearchDock, TenderResultCard } from '~/features/account/components/organisms';
import { AccountLayout } from '~/features/account/components/templates/account-layout';
import {
  loadTenderDetail,
  TenderDetailView,
  TenderNotFoundError,
  useTenderHead,
} from '~/features/tenders';

export const Route = createFileRoute('/_authenticated/tenders/$id')({
  loader: ({ params }) => loadTenderDetail(params.id),
  errorComponent: AuthedTenderNotFound,
  component: AccountTenderDetail,
});

function AccountTenderDetail() {
  const { tender, related } = Route.useLoaderData();
  const navigate = useNavigate();
  const [query, setQuery] = useState('');
  useTenderHead(tender);

  function runSearch() {
    const q = query.trim();
    if (!q) return;
    void navigate({ to: '/tenders', search: { q } });
  }

  return (
    <AccountLayout>
      <PageHeader />
      <div className="flex min-h-full flex-1 flex-col px-4 pb-28">
        <TenderDetailView
          tender={tender}
          related={related}
          renderRelated={(tr) => (
            <Link
              to="/tenders/$id"
              params={{ id: tr.id }}
              className="block rounded-2xl no-underline outline-none focus-visible:ring-2 focus-visible:ring-brand-600"
            >
              <TenderResultCard tender={tr} />
            </Link>
          )}
        />
      </div>
      {/* Operational search dock, pinned bottom-centre (matches Overview). Submitting
          routes to Explore's nuqs-backed ?q= (search runs there). */}
      <div className="pointer-events-none sticky bottom-4 z-10 flex justify-center px-4">
        <div className="pointer-events-auto w-full max-w-xl">
          <SearchDock mode="search" value={query} onChange={setQuery} onSubmit={runSearch} />
        </div>
      </div>
    </AccountLayout>
  );
}

function AuthedTenderNotFound({ error }: { error: unknown }) {
  const { t } = useTranslation();
  // Real (non-not-found) errors must not be swallowed — rethrow.
  if (!(error instanceof TenderNotFoundError)) throw error;
  return (
    <AccountLayout>
      <PageHeader />
      <div className="flex flex-1 flex-col items-center justify-center gap-4 px-4 text-center">
        <p className="text-lg text-ink-900">{t('tenders.detail.notFound')}</p>
        <Link to="/tenders" className="text-brand-700 underline">
          {t('tenders.detail.backToSearch')}
        </Link>
      </div>
    </AccountLayout>
  );
}
