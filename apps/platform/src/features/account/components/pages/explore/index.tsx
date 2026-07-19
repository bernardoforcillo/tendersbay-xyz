import { Banner, Button, cn, EmptyState } from '@tendersbay/components/core';
import { SearchX } from 'lucide-react';
import { motion, useReducedMotion } from 'motion/react';
import { useQueryState } from 'nuqs';
import { usePostHog } from 'posthog-js/react';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { SearchModeSwitch } from '~/features/account/components/molecules';
import {
  ChatWindow,
  ClientProfileForm,
  PageHeader,
  SearchDock,
  type SearchMode,
  TenderResultCard,
} from '~/features/account/components/organisms';
import { useTenderSearch } from '~/features/account/components/organisms/tender-feed';
import { AccountLayout } from '~/features/account/components/templates/account-layout';
import { useTenderLink } from '~/features/tenders';
import { useAuthStore } from '~/store/auth';
import { useChatStore } from '~/store/chat';
import { useWorkspaceStore } from '~/store/workspace';
import {
  EMPTY_FILTERS,
  type ExploreFilterKey,
  ExploreFilters,
  type FilterSelections,
  hasActiveFilters,
  toFilterValues,
} from './filters';
import { useClientShortlist } from './use-client-shortlist';

export function AccountExplorePage() {
  const { t, i18n } = useTranslation();
  const posthog = usePostHog();
  const reduce = useReducedMotion();
  const user = useAuthStore((s) => s.user);
  const name = user?.displayName?.split(' ')[0];
  const hasChats = useChatStore((s) => s.messages.length > 0 || s.currentChatId !== null);
  const hasDraft = useChatStore((s) => s.draft !== null);
  const [mode, setMode] = useState<SearchMode>(hasChats || hasDraft ? 'chat' : 'search');
  const [query, setQuery] = useQueryState('q', { defaultValue: '', clearOnDefault: true });
  const tenderLink = useTenderLink();
  const [searched, setSearched] = useState(false);
  const [filters, setFilters] = useState<FilterSelections>(EMPTY_FILTERS);
  const { results, hasMore, loading, error, search, loadMore } = useTenderSearch();
  const currentWorkspaceId = useWorkspaceStore((s) => s.currentWorkspaceId);
  const shortlist = useClientShortlist(currentWorkspaceId);

  // A palette ask can arrive while already on /explore — flip to chat so
  // ChatWindow mounts and consumes the draft.
  useEffect(() => {
    if (hasDraft) setMode('chat');
  }, [hasDraft]);

  // Fires once per distinct result set actually shown, not on every re-render.
  // Re-fires only when the actual result set changes — `posthog` is stable
  // across renders (from usePostHog()) and intentionally excluded so a
  // provider re-render alone can't re-trigger the capture.
  // biome-ignore lint/correctness/useExhaustiveDependencies: posthog is stable, intentionally excluded
  useEffect(() => {
    if (shortlist.needsProfile || shortlist.results.length === 0) return;
    posthog?.capture('client_shortlist_viewed', {
      location: 'explore_shortlist',
      shortlist_size: shortlist.results.length,
      has_strong: shortlist.results.some((r) => r.fitTier === 'strong'),
    });
  }, [shortlist.results, shortlist.needsProfile]);

  // A search runs when there is a query OR at least one active filter, so a
  // filters-only search (empty text box) is valid. loadMore reuses these filters.
  const runSearch = (selections: FilterSelections) => {
    const trimmed = query.trim();
    if (!trimmed && !hasActiveFilters(selections)) return;
    setSearched(true);
    void search(trimmed, toFilterValues(selections, new Date()), currentWorkspaceId ?? undefined);
  };

  // Seed a search when arriving with ?q= (e.g. from the detail page's dock or a shared link).
  // biome-ignore lint/correctness/useExhaustiveDependencies: run once on mount only.
  useEffect(() => {
    if (query.trim() && !searched) {
      runSearch(filters);
    }
  }, []);

  function handleSearch() {
    runSearch(filters);
  }

  function handleFilterChange(key: ExploreFilterKey, next: string) {
    const updated = { ...filters, [key]: next };
    setFilters(updated);
    posthog?.capture('explore_filter_applied', {
      filter: key,
      has_query: query.trim().length > 0,
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
        {mode === 'chat' ? (
          <>
            <div className="flex items-center justify-center pt-4 pb-2">
              <SearchModeSwitch mode={mode} onChange={setMode} />
            </div>
            <ChatWindow />
          </>
        ) : (
          <div
            className={cn(
              'flex flex-1 flex-col gap-6',
              searched ? 'pt-4' : 'items-center justify-center',
            )}
          >
            <motion.div
              initial={reduce ? false : { opacity: 0, y: 6 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.4, delay: 0.18, ease: [0.22, 1, 0.36, 1] }}
              className={cn(searched && 'flex justify-center')}
            >
              <SearchModeSwitch mode={mode} onChange={setMode} />
            </motion.div>
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
                mode={mode}
                value={query}
                onChange={(v) => void setQuery(v)}
                onSubmit={handleSearch}
              />
            </div>
            <ExploreFilters
              value={filters}
              locale={i18n.language}
              onChange={handleFilterChange}
              onClear={handleClearFilters}
            />
            {searched ? (
              <div className="mx-auto w-full max-w-xl space-y-4">
                {results.length > 0 && (
                  <>
                    <p className="text-sm text-ink-500">
                      {t('tenders.results', { count: results.length })}
                    </p>
                    <div className="space-y-3">
                      {results.map((tender) => (
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
                            />,
                            'block rounded-2xl no-underline outline-none focus-visible:ring-2 focus-visible:ring-brand-600',
                          )}
                        </div>
                      ))}
                    </div>
                    {hasMore && (
                      <div className="flex justify-center pt-2">
                        <Button
                          variant="ghost"
                          isDisabled={loading}
                          onPress={() => void loadMore()}
                        >
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
                          defaultValue:
                            'Try a manual search below, or widen this client’s profile.',
                        })}
                      />
                    )
                  )}
                  {shortlist.error && <Banner tone="error">{shortlist.error}</Banner>}
                </div>
              )
            )}
          </div>
        )}
      </div>
    </AccountLayout>
  );
}
