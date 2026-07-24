import { createFileRoute, Link } from '@tanstack/react-router';
import { Banner, Button } from '@tendersbay/components/core';
import { SearchX } from 'lucide-react';
import { useQueryState } from 'nuqs';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { TenderResultCard } from '~/features/account/components/organisms';
import { useTenderSearch } from '~/features/account/components/organisms/tender-feed';
import { Icon } from '~/features/landing/components/atoms';
import { SiteFooter, SiteHeader } from '~/features/landing/components/organisms';

export const Route = createFileRoute('/$locale/tenders/')({
  validateSearch: (search: Record<string, unknown>): { q?: string } => ({
    q: typeof search.q === 'string' ? search.q : undefined,
  }),
  component: PublicTendersPage,
});

function PublicTendersPage() {
  const { t } = useTranslation();
  const { locale } = Route.useParams();
  const [query, setQuery] = useQueryState('q', { defaultValue: '', clearOnDefault: true });
  const [searched, setSearched] = useState(false);
  const { results, hasMore, loading, error, search, loadMore } = useTenderSearch();
  const inputRef = useRef<HTMLInputElement>(null);

  const runSearch = (q = query) => {
    const trimmed = q.trim();
    if (!trimmed) return;
    setSearched(true);
    void search(trimmed);
  };

  // biome-ignore lint/correctness/useExhaustiveDependencies: run once on mount when arriving with ?q=
  useEffect(() => {
    if (query.trim() && !searched) runSearch();
  }, []);

  return (
    <div className="flex min-h-screen flex-col bg-cream-100">
      <SiteHeader />
      <main className="flex flex-1 flex-col gap-8 px-4 pt-24 pb-12">
        <div className="mx-auto w-full max-w-xl">
          <form
            onSubmit={(e) => {
              e.preventDefault();
              runSearch();
            }}
            className="flex items-center gap-3 rounded-full border border-ink-200 bg-white/80 px-5 py-3.5 shadow-soft backdrop-blur"
          >
            <Icon name="sparkle" className="shrink-0 text-[18px] text-ink-400" />
            <input
              ref={inputRef}
              type="search"
              value={query}
              onChange={(e) => void setQuery(e.target.value)}
              aria-label={t('landing.search.label')}
              placeholder={t('landing.search.label')}
              className="min-w-0 flex-1 bg-transparent text-[15px] text-ink-900 outline-none placeholder:text-ink-400"
            />
            <button
              type="submit"
              aria-label={t('shell.palette.trigger')}
              className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-brand-600 text-white outline-none transition-colors hover:bg-brand-700 active:bg-brand-800 focus-visible:ring-2 focus-visible:ring-brand-600 focus-visible:ring-offset-2 focus-visible:ring-offset-cream-100"
            >
              <Icon name="arrow-right" className="text-[16px]" />
            </button>
          </form>
        </div>

        {searched && (
          <div className="mx-auto w-full max-w-xl space-y-4">
            {results.length > 0 && (
              <>
                <p className="text-sm text-ink-500">
                  {t('tenders.results', { count: results.length })}
                </p>
                <div className="space-y-3">
                  {results.map((tender) => (
                    <Link
                      key={tender.id}
                      to="/$locale/tenders/$id"
                      params={{ locale, id: tender.id }}
                      className="block rounded-2xl no-underline outline-none focus-visible:ring-2 focus-visible:ring-brand-600"
                    >
                      <TenderResultCard tender={tender} />
                    </Link>
                  ))}
                </div>
                {hasMore && (
                  <div className="flex justify-center pt-2">
                    <Button variant="ghost" isDisabled={loading} onPress={() => void loadMore()}>
                      {t('tenders.loadMore')}
                    </Button>
                  </div>
                )}
              </>
            )}
            {error && <Banner tone="error">{t('tenders.error')}</Banner>}
            {!error &&
              results.length === 0 &&
              (loading ? (
                <p className="text-center text-sm text-ink-500">{t('tenders.searching')}</p>
              ) : (
                <div className="flex flex-col items-center gap-2 text-center">
                  <SearchX size={28} className="text-ink-400" />
                  <p className="text-ink-900">{t('tenders.empty.title')}</p>
                  <p className="text-sm text-ink-500">{t('tenders.empty.description')}</p>
                </div>
              ))}
          </div>
        )}
      </main>
      <SiteFooter />
    </div>
  );
}
