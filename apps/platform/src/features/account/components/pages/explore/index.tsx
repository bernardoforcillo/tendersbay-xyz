import { Banner, Button, cn, EmptyState } from '@tendersbay/components/core';
import { SearchX } from 'lucide-react';
import { motion, useReducedMotion } from 'motion/react';
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
import { useAuthStore } from '~/store/auth';
import { useChatStore } from '~/store/chat';

export function AccountExplorePage() {
  const { t } = useTranslation();
  const reduce = useReducedMotion();
  const user = useAuthStore((s) => s.user);
  const name = user?.displayName?.split(' ')[0];
  const hasChats = useChatStore((s) => s.messages.length > 0 || s.currentChatId !== null);
  const hasDraft = useChatStore((s) => s.draft !== null);
  const [mode, setMode] = useState<SearchMode>(hasChats || hasDraft ? 'chat' : 'search');
  const [query, setQuery] = useState('');
  const [searched, setSearched] = useState(false);
  const { results, hasMore, loading, error, search, loadMore } = useTenderSearch();

  // A palette ask can arrive while already on /explore — flip to chat so
  // ChatWindow mounts and consumes the draft.
  useEffect(() => {
    if (hasDraft) setMode('chat');
  }, [hasDraft]);

  function handleSearch() {
    const trimmed = query.trim();
    if (!trimmed) return;
    setSearched(true);
    void search(trimmed);
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
              <SearchDock mode={mode} value={query} onChange={setQuery} onSubmit={handleSearch} />
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
                        <TenderResultCard key={tender.id} tender={tender} />
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
