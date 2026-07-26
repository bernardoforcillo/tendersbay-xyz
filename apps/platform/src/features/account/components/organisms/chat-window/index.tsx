import type { TenderResult } from '@tendersbay/proto/tender/v1/tender_pb';
import { animate } from 'motion';
import { usePostHog } from 'posthog-js/react';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ToolChip } from '~/features/account/components/atoms/tool-chip';
import { ChatInput } from '~/features/account/components/molecules/chat-input';
import { CreditDisplay } from '~/features/account/components/molecules/credit-display';
import { MessageBubble } from '~/features/account/components/molecules/message-bubble';
import { useChatStream } from '~/features/account/hooks/use-chat-stream';
import { agentClient, workspaceClient } from '~/lib/api/client';
import { type ChatMessage, useChatStore } from '~/store/chat';
import { useWorkspaceStore } from '~/store/workspace';

interface ChatWindowProps {
  /** Analytics surface — 'explore' (default) or 'workbench'. */
  location?: 'explore' | 'workbench';
  /** When set, a freshly created chat is bound to this workbench. */
  workbenchId?: string;
}

export function ChatWindow({ location = 'explore', workbenchId }: ChatWindowProps) {
  const { t } = useTranslation();
  const posthog = usePostHog();
  const bottomRef = useRef<HTMLDivElement>(null);
  const messages = useChatStore((s) => s.messages);
  const streaming = useChatStore((s) => s.streaming);
  const streamingContent = useChatStore((s) => s.streamingContent);
  const activeTools = useChatStore((s) => s.activeTools);
  const currentChatId = useChatStore((s) => s.currentChatId);
  const credits = useChatStore((s) => s.credits);
  const pendingChoice = useChatStore((s) => s.pendingChoice);
  const setCurrentChat = useChatStore((s) => s.setCurrentChat);
  const setCredits = useChatStore((s) => s.setCredits);
  const workspaceId = useWorkspaceStore((s) => s.currentWorkspaceId);
  const { sendMessage, submitChoice } = useChatStream(location);
  const draft = useChatStore((s) => s.draft);
  const [creating, setCreating] = useState(false);
  const [historyReady, setHistoryReady] = useState(false);
  const loadedChatIdRef = useRef<string | null>(null);
  const sessionStartedRef = useRef(false);

  useEffect(() => {
    const parent = bottomRef.current?.parentElement;
    if (parent) {
      animate(parent, { scrollTop: parent.scrollHeight }, { duration: 0.3, ease: 'easeOut' });
    }
  });

  // A prior tab or a dropped connection can leave `streaming` truthy in memory
  // with no live stream behind it. Clear the ephemeral streaming state once on
  // mount so the input is never wedged disabled and no orphan chips linger.
  const didGuardRef = useRef(false);
  useEffect(() => {
    if (didGuardRef.current) return;
    didGuardRef.current = true;
    const s = useChatStore.getState();
    if (s.streaming) {
      s.setStreaming(false);
      s.setStreamingContent('');
      s.clearActiveTools();
    }
  }, []);

  // Fire the session-completed rollup on unmount (navigation away / chat
  // switch), then reset the session-scoped counters for the next chat.
  // posthog is a stable client instance for the component's life; this effect
  // intentionally runs its cleanup exactly once, on unmount.
  // biome-ignore lint/correctness/useExhaustiveDependencies: unmount-only rollup
  useEffect(() => {
    return () => {
      const s = useChatStore.getState();
      posthog?.capture('chat_session_completed', {
        location,
        message_count: s.messages.length,
        tool_calls_total: s.toolCallsTotal,
        tenders_opened: s.tendersOpened,
      });
      s.resetSessionCounters();
      sessionStartedRef.current = false;
    };
  }, []);

  useEffect(() => {
    if (currentChatId && workspaceId && loadedChatIdRef.current !== currentChatId) {
      loadedChatIdRef.current = currentChatId;
      setHistoryReady(false);
      agentClient
        .getMessages({ chatId: currentChatId })
        .then((res) => {
          const store = useChatStore.getState();
          // The backend is the source of truth for persisted history — replace
          // the store's messages wholesale rather than appending. A live-sent
          // user/assistant message was optimistically added with a
          // client-generated id (crypto.randomUUID(), see useChatStream), which
          // never matches the real id GetMessages returns for that same row, so
          // an append here would duplicate it even with addMessage's id dedup.
          const nextMessages: ChatMessage[] = [];
          let lastChoicePrompt: (typeof res.messages)[number] | null = null;
          for (const m of res.messages) {
            if (m.role === 'user' || m.role === 'assistant') {
              nextMessages.push({
                id: m.id,
                role: m.role,
                content: m.content,
                createdAt: m.createdAt,
              });
              lastChoicePrompt = null;
            } else if (m.role === 'choice_prompt') {
              const choices = m.choices.length
                ? (JSON.parse(new TextDecoder().decode(m.choices)) as {
                    key: string;
                    label: string;
                    description: string;
                  }[])
                : [];
              nextMessages.push({
                id: m.id,
                role: 'choice_prompt',
                content: m.content,
                createdAt: m.createdAt,
                choices,
              });
              lastChoicePrompt = m;
            } else if (m.role === 'choice_response') {
              nextMessages.push({
                id: m.id,
                role: 'choice_response',
                content: m.content,
                createdAt: m.createdAt,
              });
              lastChoicePrompt = null;
            } else if (m.role === 'tender_results') {
              const items = m.tenders.length
                ? (JSON.parse(new TextDecoder().decode(m.tenders)) as {
                    id: string;
                    title: string;
                    buyerName: string;
                    status: string;
                    country: string;
                    cpv: string;
                    value: number;
                    currency: string;
                    deadline: string;
                    source: string;
                  }[])
                : [];
              nextMessages.push({
                id: m.id,
                role: 'tender_results',
                content: '',
                createdAt: m.createdAt,
                tenders: items.map(
                  (item) =>
                    ({
                      $typeName: 'tender.v1.TenderResult',
                      id: item.id,
                      title: item.title,
                      buyerName: item.buyerName,
                      status: item.status,
                      procedureType: '',
                      country: item.country,
                      cpv: item.cpv,
                      value: BigInt(item.value),
                      currency: item.currency,
                      publishedAt: '',
                      deadline: item.deadline,
                      relevanceScore: 0,
                      source: item.source,
                      sourceRef: '',
                      sourceUrl: '',
                    }) as TenderResult,
                ),
              });
              lastChoicePrompt = null;
            }
          }
          store.setMessages(nextMessages);
          if (lastChoicePrompt) {
            const choices = JSON.parse(new TextDecoder().decode(lastChoicePrompt.choices)) as {
              key: string;
              label: string;
              description: string;
            }[];
            store.setPendingChoice({
              id: lastChoicePrompt.id,
              question: lastChoicePrompt.content,
              options: choices,
              allowCustom: false,
            });
          } else {
            store.setPendingChoice(null);
          }
          setHistoryReady(true);
        })
        .catch(() => {
          setHistoryReady(true);
        });
    } else {
      setHistoryReady(true);
    }
  }, [currentChatId, workspaceId]);

  useEffect(() => {
    if (workspaceId) {
      agentClient
        .getCredits({ workspaceId })
        .then((res) => {
          setCredits({
            remaining: Number(res.remaining),
            monthlyMax: Number(res.monthlyMax),
            used: Number(res.used),
            resetDate: res.resetDate,
          });
        })
        .catch(() => {});
    }
  }, [workspaceId, setCredits]);

  // Consume a palette draft once history is settled: for an existing chat the
  // reload's wholesale setMessages would wipe the optimistic message this
  // send adds (same race handleSend pre-marks loadedChatIdRef for new chats).
  // biome-ignore lint/correctness/useExhaustiveDependencies: handleSend is a stable function declaration
  useEffect(() => {
    if (!draft || !historyReady) return;
    useChatStore.getState().setDraft(null);
    void handleSend(draft);
  }, [draft, historyReady]);

  async function handleSend(message: string) {
    let chatId = currentChatId;

    if (!chatId) {
      if (!workspaceId) return;
      setCreating(true);
      try {
        const res = await agentClient.createChat({
          workspaceId,
          workbenchId,
          agentType: 'base-chat',
        });
        chatId = res.chat?.id ?? null;
        if (chatId) {
          setCurrentChat(chatId);
          // A brand-new chat has no history to restore. Mark it as already
          // loaded so the reload effect doesn't fire a concurrent
          // GetMessages call that could race the very first sendMessage
          // below and win with a stale (still-empty) response, wiping the
          // just-rendered optimistic user message via setMessages' now-
          // destructive replace.
          loadedChatIdRef.current = chatId;
        }
      } catch {
        return;
      } finally {
        setCreating(false);
      }
    }

    if (!chatId) return;

    if (!sessionStartedRef.current) {
      sessionStartedRef.current = true;
      void workspaceClient
        .getClientProfile({ workspaceId: workspaceId ?? '' })
        .then((res) => {
          const p = res.profile;
          const hasProfile = res.exists;
          posthog?.capture('chat_session_started', {
            location,
            has_workspace: Boolean(workspaceId),
            has_client_profile: hasProfile,
          });
          if (hasProfile && p) {
            posthog?.capture('client_profile_injected', {
              location: 'explore_chat',
              has_sectors: (p.sectors?.length ?? 0) > 0,
              has_countries: (p.countries?.length ?? 0) > 0,
              value_range_set: p.valueMin !== 0n || p.valueMax !== 0n,
            });
          }
        })
        .catch(() => {
          posthog?.capture('chat_session_started', {
            location,
            has_workspace: Boolean(workspaceId),
            has_client_profile: false,
          });
        });
    }

    await sendMessage(chatId, message);

    if (workspaceId) {
      agentClient
        .getCredits({ workspaceId })
        .then((res) => {
          setCredits({
            remaining: Number(res.remaining),
            monthlyMax: Number(res.monthlyMax),
            used: Number(res.used),
            resetDate: res.resetDate,
          });
        })
        .catch(() => {});
    }
  }

  async function handleSubmitChoice(choiceId: string, selectedKey: string, customValue?: string) {
    await submitChoice(choiceId, selectedKey, customValue);
  }

  const hasMessages = messages.length > 0 || streaming;

  return (
    <div className="mx-auto flex w-full max-w-2xl flex-1 flex-col">
      {credits && (
        <div className="px-4 pb-2 pt-2">
          <CreditDisplay
            remaining={credits.remaining}
            monthlyMax={credits.monthlyMax}
            inputTokens={0}
            outputTokens={0}
          />
        </div>
      )}

      {hasMessages ? (
        <div className="flex-1 space-y-4 overflow-y-auto px-4 py-6">
          {messages.map((msg) => (
            <MessageBubble
              key={msg.id}
              message={msg}
              isPendingChoice={pendingChoice?.id === msg.id}
              onSubmitChoice={handleSubmitChoice}
            />
          ))}
          {streaming && activeTools.length > 0 && (
            <div className="flex flex-wrap gap-2">
              {activeTools.map((tool) => (
                <ToolChip key={tool.name} name={tool.name} status={tool.status} />
              ))}
            </div>
          )}
          {streaming && streamingContent && (
            <div className="flex justify-start">
              <div className="max-w-[80%] rounded-2xl bg-cream-200 px-4 py-2.5 text-sm leading-relaxed text-ink-900">
                {streamingContent}
                <span className="ml-0.5 animate-pulse">&#9612;</span>
              </div>
            </div>
          )}
          <div ref={bottomRef} />
        </div>
      ) : (
        <div className="flex flex-1 flex-col items-center justify-center gap-3 px-4">
          <p className="text-center text-sm text-ink-500">
            {t('account.explore.chatEmpty', {
              defaultValue: 'Ask anything about European tenders, deadlines, or opportunities.',
            })}
          </p>
        </div>
      )}

      <div className="border-t border-ink-100 p-4">
        <ChatInput
          onSend={handleSend}
          disabled={streaming || creating || pendingChoice !== null}
          placeholder={
            creating
              ? t('account.explore.creatingChat', { defaultValue: 'Creating chat…' })
              : undefined
          }
        />
      </div>
    </div>
  );
}
