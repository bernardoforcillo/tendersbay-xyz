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
import {
  EMPTY_FILTERS,
  type ExploreFilterKey,
  ExploreFilters,
  type FilterSelections,
  hasActiveFilters,
  toFilterValues,
} from './filters';

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

  // A palette ask can arrive while already on /explore — flip to chat so
  // ChatWindow mounts and consumes the draft.
  useEffect(() => {
    if (hasDraft) setMode('chat');
  }, [hasDraft]);

  // A search runs when there is a query OR at least one active filter, so a
  // filters-only search (empty text box) is valid. loadMore reuses these filters.
  const runSearch = (selections: FilterSelections) => {
    const trimmed = query.trim();
    if (!trimmed && !hasActiveFilters(selections)) return;
    setSearched(true);
    void search(trimmed, toFilterValues(selections, new Date()));
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
            {searched && (
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
                            <TenderResultCard tender={tender} />,
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
            )}
          </div>
        )}
      </div>
    </AccountLayout>
  );
}
