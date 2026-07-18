import { useNavigate } from '@tanstack/react-router';
import { Button, Card, EmptyState } from '@tendersbay/components/core';
import { MessageSquare, Sparkles } from 'lucide-react';
import { Button as RACButton } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { PageHeader, SearchDock, TenderResultCard } from '~/features/account/components/organisms';
import { useWorkspaceChats } from '~/features/account/hooks/use-workspace-chats';
import { FirstRunProfile } from '~/features/workspace/components/organisms/first-run-profile';
import { useWorkspaceContext } from '~/features/workspace/context';
import { useAuthStore } from '~/store/auth';
import { useChatStore } from '~/store/chat';
import { greetingKey } from './greeting';
import { useRecommendedTenders } from './use-recommended-tenders';

const RESUME_ROW =
  'flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-left text-sm text-ink-700 outline-none ' +
  'transition-colors duration-150 data-[hovered]:bg-cream-100 data-[hovered]:text-ink-900 ' +
  'data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600';

const SEE_ALL_LINK =
  'shrink-0 rounded text-xs font-medium text-brand-700 outline-none transition-colors ' +
  'data-[hovered]:text-brand-800 data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600';

export function WorkspaceTodayPage() {
  const { workspace, workspaceId } = useWorkspaceContext();
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const user = useAuthStore((s) => s.user);
  const setCurrentChat = useChatStore((s) => s.setCurrentChat);
  const setMessages = useChatStore((s) => s.setMessages);
  const setPendingChoice = useChatStore((s) => s.setPendingChoice);
  const { data: chats } = useWorkspaceChats(workspace.id);
  const { tenders } = useRecommendedTenders();

  const name = user?.displayName?.split(' ')[0];
  const period = greetingKey(new Date().getHours());
  const greeting = name
    ? t(`today.greeting.${period}Named`, { defaultValue: `Good ${period}, {{name}}.`, name })
    : t(`today.greeting.${period}`, { defaultValue: `Good ${period}.` });
  const dateLine = new Intl.DateTimeFormat(i18n.language, {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
  }).format(new Date());

  const recent = (chats ?? []).slice(0, 3);

  function resume(chatId: string) {
    // Clear the previous chat's residue so ChatWindow never renders another
    // conversation's transcript while (or if) the resumed history loads.
    setMessages([]);
    setPendingChoice(null);
    setCurrentChat(chatId);
    void navigate({ to: '/explore' });
  }

  return (
    <div className="flex min-h-full flex-col">
      <PageHeader />
      <FirstRunProfile workspaceId={workspaceId}>
        <div className="mx-auto flex w-full max-w-2xl flex-1 flex-col gap-6 px-4 pt-8 pb-4">
          <div>
            <h1 className="font-display text-3xl text-ink-900 sm:text-4xl">{greeting}</h1>
            <p className="mt-1 text-sm text-ink-500 first-letter:uppercase">{dateLine}</p>
          </div>

          {recent.length > 0 && (
            <Card padding="none" className="p-2">
              <p className="px-3 pt-2 pb-1 font-mono text-[10px] font-semibold uppercase tracking-wide text-ink-400">
                {t('today.resume.title', 'Pick up where you left off')}
              </p>
              <ul>
                {recent.map((chat) => (
                  <li key={chat.id}>
                    <RACButton onPress={() => resume(chat.id)} className={RESUME_ROW}>
                      <MessageSquare
                        size={15}
                        aria-hidden="true"
                        className="shrink-0 text-ink-400"
                      />
                      <span className="truncate">
                        {chat.title || t('today.resume.untitled', 'Untitled conversation')}
                      </span>
                    </RACButton>
                  </li>
                ))}
              </ul>
            </Card>
          )}

          {tenders.length > 0 ? (
            <section className="space-y-3">
              <div className="flex items-baseline justify-between gap-3">
                <h2 className="font-display text-xl text-ink-900">
                  {t('today.recommended.title', 'Recommended for you')}
                </h2>
                <RACButton
                  onPress={() => void navigate({ to: '/explore' })}
                  className={SEE_ALL_LINK}
                >
                  {t('today.recommended.seeAll', 'All in Explore →')}
                </RACButton>
              </div>
              <div className="space-y-3">
                {tenders.map((tender) => (
                  <TenderResultCard key={tender.id} tender={tender} />
                ))}
              </div>
            </section>
          ) : (
            <EmptyState
              icon={<Sparkles size={28} />}
              title={t('today.explore.title', 'Find your next tender')}
              description={t(
                'today.explore.description',
                'Ask the agent about markets, requirements, or a specific call — personalised recommendations will appear here as your search profiles take shape.',
              )}
              action={
                <Button variant="ghost" onPress={() => void navigate({ to: '/explore' })}>
                  {t('today.explore.action', 'Open Explore')}
                </Button>
              }
            />
          )}
        </div>
        <div className="mt-auto flex justify-center px-4 pb-6 pt-4">
          <SearchDock onPress={() => void navigate({ to: '/explore' })} />
        </div>
      </FirstRunProfile>
    </div>
  );
}
