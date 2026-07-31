import { useNavigate } from '@tanstack/react-router';
import { Banner, Button, cn, EmptyState } from '@tendersbay/components/core';
import { SearchX } from 'lucide-react';
import { motion, useReducedMotion } from 'motion/react';
import { useQueryState } from 'nuqs';
import { usePostHog } from 'posthog-js/react';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AppliedCpvChips, AppliedFilterChips } from '~/features/account/components/molecules';
import {
  ClientProfileForm,
  PageHeader,
  SearchDock,
  TenderResultCard,
} from '~/features/account/components/organisms';
import { cpvPrefix, useTenderSearch } from '~/features/account/components/organisms/tender-feed';
import {
  EMPTY_FILTERS,
  type ExploreFilterKey,
  ExploreFilters,
  type FilterSelections,
  hasActiveFilters,
  toFilterValues,
} from '~/features/account/components/pages/explore/filters';
import { useClientShortlist } from '~/features/account/components/pages/explore/use-client-shortlist';
import { AccountLayout } from '~/features/account/components/templates/account-layout';
import { useTenderLink } from '~/features/tenders';
import { useAuthStore } from '~/store/auth';
import { useChatStore } from '~/store/chat';
import { useWorkspaceStore } from '~/store/workspace';

export function AccountTendersPage() {
  const { t, i18n } = useTranslation();
  const posthog = usePostHog();
  const reduce = useReducedMotion();
  const navigate = useNavigate();
  const user = useAuthStore((s) => s.user);
  const name = user?.displayName?.split(' ')[0];
  const hasDraft = useChatStore((s) => s.draft !== null);
  const [query, setQuery] = useQueryState('q', { defaultValue: '', clearOnDefault: true });
  const tenderLink = useTenderLink();
  const [searched, setSearched] = useState(false);
  // The last query actually run, so a re-run with new filters can be told
  // apart from a brand-new search.
  const lastQueryRef = useRef('');
  const [filters, setFilters] = useState<FilterSelections>(EMPTY_FILTERS);
  // Codes the user removed from the inferred set. Client-side because the server
  // re-infers them from the same text on every request.
  const [suppressedCpv, setSuppressedCpv] = useState<string[]>([]);
  const { results, hasMore, loading, error, meta, search, loadMore } = useTenderSearch();
  const currentWorkspaceId = useWorkspaceStore((s) => s.currentWorkspaceId);
  const shortlist = useClientShortlist(currentWorkspaceId);

  // A palette ask draft arriving on the search page routes to explore (chat).
  useEffect(() => {
    if (hasDraft) void navigate({ to: '/explore' });
  }, [hasDraft, navigate]);

  // biome-ignore lint/correctness/useExhaustiveDependencies: posthog is stable, intentionally excluded
  useEffect(() => {
    if (shortlist.needsProfile || shortlist.results.length === 0) return;
    posthog?.capture('client_shortlist_viewed', {
      location: 'explore_shortlist',
      shortlist_size: shortlist.results.length,
      has_strong: shortlist.results.some((r) => r.fitTier === 'strong'),
    });
  }, [shortlist.results, shortlist.needsProfile]);

  const runSearch = (selections: FilterSelections, suppressed: string[] = suppressedCpv) => {
    const trimmed = query.trim();
    if (!trimmed && !hasActiveFilters(selections)) return;
    // A repeat of the same query with the filters changed is a REFINEMENT, not
    // a fresh search. Distinguishing them is the point: a refinement means the
    // first answer wasn't good enough, which is the signal that says whether
    // ranking changes actually help.
    const refined = lastQueryRef.current === trimmed && searched;
    // A new query text is a new inference; carrying the previous suppressions
    // forward would narrow a search the user never narrowed.
    const carried = refined ? suppressed : [];
    lastQueryRef.current = trimmed;
    setSearched(true);
    if (carried !== suppressed) setSuppressedCpv(carried);
    posthog?.capture(refined ? 'search_refined' : 'search_performed', {
      location: 'explore',
      has_query: trimmed.length > 0,
      query_length: trimmed.length,
      has_filters: hasActiveFilters(selections),
      sort: selections.sort,
      suppressed_cpv_count: carried.length,
    });
    // The fifth argument is omitted rather than passed as `[]`: the hook only
    // sends `suppressedCpv` on the wire when there is something to suppress,
    // and several existing tests assert `search`'s call arguments exactly.
    if (carried.length > 0) {
      void search(
        trimmed,
        toFilterValues(selections, new Date()),
        currentWorkspaceId ?? undefined,
        selections.sort,
        carried,
      );
    } else {
      void search(
        trimmed,
        toFilterValues(selections, new Date()),
        currentWorkspaceId ?? undefined,
        selections.sort,
      );
    }
  };

  // biome-ignore lint/correctness/useExhaustiveDependencies: run once on mount only.
  useEffect(() => {
    if (query.trim() && !searched) {
      runSearch(filters);
    }
  }, []);

  // Reported once a result set has settled, never mid-flight — a search still
  // loading has zero results for reasons that have nothing to do with the
  // query. `mode` says which retrievers answered, so a spike in empty results
  // can be told apart from a spike in outages.
  // biome-ignore lint/correctness/useExhaustiveDependencies: posthog is stable, intentionally excluded
  useEffect(() => {
    if (!searched || loading || error) return;
    if (results.length > 0) return;
    posthog?.capture('search_zero_results', {
      location: 'explore',
      mode: meta.mode,
      degraded: meta.degraded,
      has_filters: hasActiveFilters(filters),
    });
  }, [searched, loading, error, results.length, meta.mode, meta.degraded]);

  function handleSearch() {
    runSearch(filters);
  }

  function handleFilterChange(key: ExploreFilterKey, next: string[] | string) {
    const updated = { ...filters, [key]: next } as FilterSelections;
    setFilters(updated);
    posthog?.capture('explore_filter_applied', {
      filter: key,
      // How many values are active on this facet, not which — keeps the
      // property low-cardinality while still showing multi-select adoption.
      selected_count: Array.isArray(next) ? next.length : next ? 1 : 0,
      has_query: query.trim().length > 0,
      location: 'explore_filters',
    });
    runSearch(updated);
  }

  function handleClearFilters() {
    setFilters(EMPTY_FILTERS);
    runSearch(EMPTY_FILTERS);
  }

  return (
    <AccountLayout>
      <PageHeader />
      <div className="flex min-h-full flex-1 flex-col px-4 pb-16">
        <div
          className={cn(
            'flex flex-1 flex-col gap-6',
            searched ? 'pt-4' : 'items-center justify-center',
          )}
        >
          <motion.h1
            initial={reduce ? false : { opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.45, delay: 0.12, ease: [0.22, 1, 0.36, 1] }}
            className="text-center text-2xl font-semibold text-ink-900 sm:text-3xl"
          >
            {name
              ? t('account.explore.greetingNamed', {
                  defaultValue: 'What are you bidding on today, {{name}}?',
                  name,
                })
              : t('account.explore.greeting', { defaultValue: 'What are you bidding on today?' })}
          </motion.h1>
          <div className="flex w-full justify-center">
            <SearchDock
              mode="search"
              value={query}
              onChange={(v) => void setQuery(v)}
              onSubmit={handleSearch}
            />
          </div>
          <ExploreFilters
            value={filters}
            locale={i18n.language}
            facets={{
              countries: meta.countryFacets,
              statuses: meta.statusFacets,
              cpvDivisions: meta.cpvDivisionFacets,
            }}
            onChange={handleFilterChange}
            onClear={handleClearFilters}
          />
          {searched ? (
            <div className="mx-auto w-full max-w-2xl space-y-4">
              <AppliedCpvChips
                matches={meta.appliedCpv}
                onRemove={(code) => {
                  const next = [...suppressedCpv, code];
                  setSuppressedCpv(next);
                  posthog?.capture('search_cpv_suppressed', { location: 'explore', code });
                  runSearch(filters, next);
                }}
              />
              <AppliedFilterChips
                applied={meta.appliedFilters}
                explicit={{
                  countries: filters.countries,
                  cpvPrefixes: filters.sectors
                    .map(cpvPrefix)
                    .filter((p): p is string => Boolean(p)),
                  statuses: filters.statuses,
                  hasValueBounds: Boolean(filters.valueMin || filters.valueMax),
                  hasDeadline: Boolean(filters.deadline),
                }}
                locale={i18n.language}
                onClear={() => {
                  void setQuery('');
                  runSearch(filters);
                }}
              />
              {/* A search running on one retriever answers the question asked,
                  just less well — say so rather than presenting partial
                  results as complete ones. */}
              {meta.degraded && <Banner tone="warning">{t('tenders.degraded')}</Banner>}
              {results.length > 0 && (
                <>
                  <p className="text-sm text-ink-500">
                    {t('tenders.results', { count: results.length })}
                  </p>
                  <div className="space-y-3">
                    {results.map((tender, index) => (
                      <div key={tender.id}>
                        {tenderLink(
                          tender.id,
                          <TenderResultCard
                            tender={tender}
                            fitTier={
                              tender.fitTier
                                ? (tender.fitTier as 'strong' | 'possible' | 'long_shot')
                                : undefined
                            }
                            reason={tender.fitTier ? tender.reason : undefined}
                            snippet={tender.snippet}
                          />,
                          'block rounded-2xl no-underline outline-none focus-visible:ring-2 focus-visible:ring-brand-600',
                          () =>
                            posthog?.capture('search_result_clicked', {
                              location: 'explore',
                              // Position is what makes ranking measurable: a
                              // change that moves clicks up the list is the
                              // only evidence that it improved relevance.
                              position: index,
                              mode: meta.mode,
                              has_snippet: Boolean(tender.snippet),
                            }),
                        )}
                      </div>
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
                  <EmptyState
                    icon={<SearchX size={28} />}
                    title={t('tenders.empty.title')}
                    description={t('tenders.empty.description')}
                  />
                ))}
            </div>
          ) : (
            currentWorkspaceId && (
              <div className="mx-auto w-full max-w-xl space-y-4">
                {shortlist.needsProfile ? (
                  <ClientProfileForm
                    workspaceId={currentWorkspaceId}
                    onSaved={() => shortlist.refetch()}
                  />
                ) : shortlist.results.length > 0 ? (
                  <>
                    <p className="text-sm text-ink-500">
                      {t('explore.shortlist.title', { defaultValue: 'Best fit for this client' })}
                    </p>
                    <div className="space-y-3">
                      {shortlist.results.map((r) => (
                        <div key={r.tender?.id}>
                          {tenderLink(
                            r.tender?.id ?? '',
                            <TenderResultCard
                              tender={r.tender as NonNullable<typeof r.tender>}
                              fitTier={r.fitTier as 'strong' | 'possible' | 'long_shot'}
                              reason={r.reason as NonNullable<typeof r.reason>}
                            />,
                            'block rounded-2xl no-underline outline-none focus-visible:ring-2 focus-visible:ring-brand-600',
                            () =>
                              posthog?.capture('shortlist_match_opened', {
                                location: 'explore_shortlist',
                                fit_tier: r.fitTier,
                              }),
                          )}
                        </div>
                      ))}
                    </div>
                  </>
                ) : (
                  !shortlist.loading && (
                    <EmptyState
                      icon={<SearchX size={28} />}
                      title={t('explore.shortlist.emptyTitle', {
                        defaultValue: 'No best-fit tenders yet',
                      })}
                      description={t('explore.shortlist.emptyDescription', {
                        defaultValue: "Try a manual search below, or widen this client's profile.",
                      })}
                    />
                  )
                )}
                {shortlist.error && <Banner tone="error">{shortlist.error}</Banner>}
              </div>
            )
          )}
        </div>
      </div>
    </AccountLayout>
  );
}
