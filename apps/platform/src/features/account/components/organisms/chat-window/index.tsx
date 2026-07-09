import { animate } from 'motion';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ChatInput } from '~/features/account/components/molecules/chat-input';
import { CreditDisplay } from '~/features/account/components/molecules/credit-display';
import { MessageBubble } from '~/features/account/components/molecules/message-bubble';
import { useChatStream } from '~/features/account/hooks/use-chat-stream';
import { agentClient } from '~/lib/api/client';
import { useChatStore } from '~/store/chat';
import { useWorkspaceStore } from '~/store/workspace';

export function ChatWindow() {
  const { t } = useTranslation();
  const bottomRef = useRef<HTMLDivElement>(null);
  const messages = useChatStore((s) => s.messages);
  const streaming = useChatStore((s) => s.streaming);
  const streamingContent = useChatStore((s) => s.streamingContent);
  const currentChatId = useChatStore((s) => s.currentChatId);
  const credits = useChatStore((s) => s.credits);
  const setCurrentChat = useChatStore((s) => s.setCurrentChat);
  const setCredits = useChatStore((s) => s.setCredits);
  const workspaceId = useWorkspaceStore((s) => s.currentWorkspaceId);
  const { sendMessage } = useChatStream();
  const [creating, setCreating] = useState(false);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    const parent = bottomRef.current?.parentElement;
    if (parent) {
      animate(parent, { scrollTop: parent.scrollHeight }, { duration: 0.3, ease: 'easeOut' });
    }
  });

  useEffect(() => {
    if (currentChatId && workspaceId && !loaded) {
      setLoaded(true);
      agentClient
        .getMessages({ chatId: currentChatId })
        .then((res) => {
          const store = useChatStore.getState();
          for (const m of res.messages) {
            if (m.role === 'user' || m.role === 'assistant') {
              store.addMessage({
                id: m.id,
                role: m.role,
                content: m.content,
                createdAt: m.createdAt,
              });
            }
          }
        })
        .catch(() => {});
    }
  }, [currentChatId, workspaceId, loaded]);

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

  async function handleSend(message: string) {
    let chatId = currentChatId;

    if (!chatId) {
      if (!workspaceId) return;
      setCreating(true);
      try {
        const res = await agentClient.createChat({
          workspaceId,
          agentType: 'base-chat',
        });
        chatId = res.chat?.id ?? null;
        if (chatId) {
          setCurrentChat(chatId);
          setLoaded(false);
        }
      } catch {
        return;
      } finally {
        setCreating(false);
      }
    }

    if (!chatId) return;

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
            <MessageBubble key={msg.id} message={msg} />
          ))}
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
          disabled={streaming || creating}
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
