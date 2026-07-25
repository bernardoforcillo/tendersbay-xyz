import { usePostHog } from 'posthog-js/react';
import { ChoicePromptCard } from '~/features/account/components/molecules/choice-prompt-card';
import { TenderResultsTable } from '~/features/account/components/organisms/tender-results-table';
import { type ChatMessage, useChatStore } from '~/store/chat';

type MessageBubbleProps = {
  message: ChatMessage;
  isPendingChoice: boolean;
  onSubmitChoice: (choiceId: string, selectedKey: string, customValue?: string) => void;
};

export function MessageBubble({ message, isPendingChoice, onSubmitChoice }: MessageBubbleProps) {
  const posthog = usePostHog();

  if (message.role === 'choice_prompt') {
    return (
      <ChoicePromptCard
        message={message}
        isPending={isPendingChoice}
        onSubmit={(selectedKey, customValue) =>
          onSubmitChoice(message.id, selectedKey, customValue)
        }
      />
    );
  }

  if (message.role === 'tender_results') {
    return (
      <div className="flex justify-start">
        <div className="w-full">
          <TenderResultsTable
            tenders={message.tenders ?? []}
            onOpen={(tender) => {
              useChatStore.getState().incTendersOpened();
              posthog?.capture('chat_tender_card_opened', {
                location: 'explore_chat',
                // Chat search is client-agnostic, so a card carries no fit tier.
                fit_tier: tender.fitTier || 'none',
              });
            }}
          />
        </div>
      </div>
    );
  }

  const isUser = message.role === 'user' || message.role === 'choice_response';

  return (
    <div className={`flex ${isUser ? 'justify-end' : 'justify-start'}`}>
      <div
        className={`max-w-[80%] rounded-2xl px-4 py-2.5 text-sm leading-relaxed ${
          isUser ? 'bg-brand-600 text-white' : 'bg-cream-200 text-ink-900'
        }`}
      >
        {message.content}
      </div>
    </div>
  );
}
