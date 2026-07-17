import { createFileRoute, Link, useNavigate } from '@tanstack/react-router';
import { useState } from 'react';
import { PageHeader, SearchDock, TenderResultCard } from '~/features/account/components/organisms';
import { AccountLayout } from '~/features/account/components/templates/account-layout';
import { loadTenderDetail, TenderDetailView, useTenderHead } from '~/features/tenders';

export const Route = createFileRoute('/_authenticated/explore/tenders/$id')({
  loader: ({ params }) => loadTenderDetail(params.id),
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
    void navigate({ to: '/explore', search: { q } });
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
              to="/explore/tenders/$id"
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
