import { createFileRoute, Link } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import { TenderResultCard } from '~/features/account/components/organisms';
import { SiteFooter, SiteHeader } from '~/features/landing/components/organisms';
import {
  loadTenderDetail,
  TenderDetailView,
  TenderNotFoundError,
  useTenderHead,
} from '~/features/tenders';

export const Route = createFileRoute('/$locale/tenders/$id')({
  loader: ({ params }) => loadTenderDetail(params.id),
  errorComponent: TenderNotFound,
  component: PublicTenderDetail,
});

function PublicTenderDetail() {
  const { tender, related } = Route.useLoaderData();
  const { locale } = Route.useParams();
  useTenderHead(tender);
  return (
    <div className="flex min-h-screen flex-col bg-cream-100">
      <SiteHeader />
      <main className="flex-1 px-4 pt-24">
        <TenderDetailView
          tender={tender}
          related={related}
          renderRelated={(tr) => (
            <Link
              to="/$locale/tenders/$id"
              params={{ locale, id: tr.id }}
              className="block rounded-2xl no-underline outline-none focus-visible:ring-2 focus-visible:ring-brand-600"
            >
              <TenderResultCard tender={tr} />
            </Link>
          )}
        />
      </main>
      <SiteFooter />
    </div>
  );
}

function TenderNotFound({ error }: { error: unknown }) {
  const { t } = useTranslation();
  // Real (non-not-found) errors must not be swallowed — rethrow so the router's
  // default/boundary handling applies.
  if (!(error instanceof TenderNotFoundError)) throw error;
  const { locale } = Route.useParams();
  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-4 bg-cream-100 px-4 text-center">
      <p className="text-lg text-ink-900">{t('tenders.detail.notFound')}</p>
      <Link to="/$locale" params={{ locale }} className="text-brand-700 underline">
        {t('tenders.detail.backToSearch')}
      </Link>
    </div>
  );
}
