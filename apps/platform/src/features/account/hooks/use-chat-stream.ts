import { useCallback, useEffect, useRef } from 'react';
import type { CreditsData } from '~/features/account/components/molecules/credit-display';
import { agentClient } from '~/lib/api/client';
import { useChatStore } from '~/store/chat';

export function useChatStream() {
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    return () => {
      abortRef.current?.abort();
    };
  }, []);

  const cancel = useCallback(() => {
    abortRef.current?.abort();
    useChatStore.getState().setStreaming(false);
  }, []);

  const sendMessage = useCallback(
    async (chatId: string, message: string): Promise<CreditsData | null> => {
      abortRef.current?.abort();
      const abort = new AbortController();
      abortRef.current = abort;

      const store = useChatStore.getState();
      store.addMessage({
        id: crypto.randomUUID(),
        role: 'user',
        content: message,
        createdAt: new Date().toISOString(),
      });
      store.setStreaming(true);
      store.setStreamingContent('');

      try {
        const stream = agentClient.chatStream({ chatId, message }, { signal: abort.signal });
        let fullContent = '';

        for await (const response of stream) {
          if (abort.signal.aborted) break;

          const event = response.event;
          if (event.case === 'token') {
            fullContent += event.value;
            useChatStore.getState().appendStreamToken(event.value);
          } else if (event.case === 'choice') {
            const options = event.value.options.map((o) => ({
              key: o.key,
              label: o.label,
              description: o.description,
            }));
            useChatStore.getState().setStreaming(false);
            useChatStore.getState().setStreamingContent('');
            useChatStore.getState().addMessage({
              id: event.value.id,
              role: 'choice_prompt',
              content: event.value.question,
              createdAt: new Date().toISOString(),
              choices: options,
            });
            useChatStore.getState().setPendingChoice({
              id: event.value.id,
              question: event.value.question,
              options,
              allowCustom: event.value.allowCustom,
            });
            return null;
          } else if (event.case === 'done') {
            if (fullContent) {
              useChatStore.getState().addMessage({
                id: crypto.randomUUID(),
                role: 'assistant',
                content: fullContent,
                createdAt: new Date().toISOString(),
              });
            }
            useChatStore.getState().setStreamingContent('');
            useChatStore.getState().setStreaming(false);

            const done = event.value;
            const credits: CreditsData = {
              remaining: Number(done.creditsRemaining),
              monthlyMax: Number(done.creditsMonthlyMax),
              inputTokens: Number(done.usage?.totalTokens ?? 0),
              outputTokens: Number(done.usage?.outputTokens ?? 0),
            };
            useChatStore.getState().setCredits({
              remaining: Number(done.creditsRemaining),
              monthlyMax: Number(done.creditsMonthlyMax),
              used: Number(done.creditsMonthlyMax) - Number(done.creditsRemaining),
              resetDate: '',
            });
            return credits;
          } else if (event.case === 'error') {
            useChatStore.getState().setStreaming(false);
            useChatStore.getState().setStreamingContent('');
            throw new Error(event.value.message);
          }
        }
      } catch (e: unknown) {
        if (abort.signal.aborted) return null;
        useChatStore.getState().setStreaming(false);
        useChatStore.getState().setStreamingContent('');
        useChatStore.getState().addMessage({
          id: crypto.randomUUID(),
          role: 'assistant',
          content: e instanceof Error ? e.message : 'Something went wrong',
          createdAt: new Date().toISOString(),
        });
        return null;
      }

      return null;
    },
    [],
  );

  const submitChoice = useCallback(
    async (
      choiceId: string,
      selectedKey: string,
      customValue = '',
    ): Promise<CreditsData | null> => {
      abortRef.current?.abort();
      const abort = new AbortController();
      abortRef.current = abort;

      const store = useChatStore.getState();
      store.setPendingChoice(null);
      store.setStreaming(true);
      store.setStreamingContent('');

      try {
        const stream = agentClient.submitChoice(
          { choiceId, selectedKey, customValue },
          { signal: abort.signal },
        );
        let fullContent = '';

        for await (const response of stream) {
          if (abort.signal.aborted) break;

          const event = response.event;
          if (event.case === 'token') {
            fullContent += event.value;
            useChatStore.getState().appendStreamToken(event.value);
          } else if (event.case === 'choice') {
            const options = event.value.options.map((o) => ({
              key: o.key,
              label: o.label,
              description: o.description,
            }));
            useChatStore.getState().setStreaming(false);
            useChatStore.getState().setStreamingContent('');
            useChatStore.getState().addMessage({
              id: event.value.id,
              role: 'choice_prompt',
              content: event.value.question,
              createdAt: new Date().toISOString(),
              choices: options,
            });
            useChatStore.getState().setPendingChoice({
              id: event.value.id,
              question: event.value.question,
              options,
              allowCustom: event.value.allowCustom,
            });
            return null;
          } else if (event.case === 'done') {
            if (fullContent) {
              useChatStore.getState().addMessage({
                id: crypto.randomUUID(),
                role: 'assistant',
                content: fullContent,
                createdAt: new Date().toISOString(),
              });
            }
            useChatStore.getState().setStreamingContent('');
            useChatStore.getState().setStreaming(false);

            const done = event.value;
            const credits: CreditsData = {
              remaining: Number(done.creditsRemaining),
              monthlyMax: Number(done.creditsMonthlyMax),
              inputTokens: Number(done.usage?.totalTokens ?? 0),
              outputTokens: Number(done.usage?.outputTokens ?? 0),
            };
            useChatStore.getState().setCredits({
              remaining: Number(done.creditsRemaining),
              monthlyMax: Number(done.creditsMonthlyMax),
              used: Number(done.creditsMonthlyMax) - Number(done.creditsRemaining),
              resetDate: '',
            });
            return credits;
          } else if (event.case === 'error') {
            useChatStore.getState().setStreaming(false);
            useChatStore.getState().setStreamingContent('');
            throw new Error(event.value.message);
          }
        }
      } catch (e: unknown) {
        if (abort.signal.aborted) return null;
        useChatStore.getState().setStreaming(false);
        useChatStore.getState().setStreamingContent('');
        useChatStore.getState().addMessage({
          id: crypto.randomUUID(),
          role: 'assistant',
          content: e instanceof Error ? e.message : 'Something went wrong',
          createdAt: new Date().toISOString(),
        });
        return null;
      }

      return null;
    },
    [],
  );

  return { sendMessage, submitChoice, cancel };
}
