import { ChoicePromptCard } from '~/features/account/components/molecules/choice-prompt-card';
import { TenderResultCard } from '~/features/account/components/organisms/tender-result-card';
import type { ChatMessage } from '~/store/chat';

type MessageBubbleProps = {
  message: ChatMessage;
  isPendingChoice: boolean;
  onSubmitChoice: (choiceId: string, selectedKey: string, customValue?: string) => void;
};

export function MessageBubble({ message, isPendingChoice, onSubmitChoice }: MessageBubbleProps) {
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
        <div className="w-full max-w-[80%] space-y-2.5">
          {(message.tenders ?? []).map((tender) => (
            <TenderResultCard key={tender.id} tender={tender} />
          ))}
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
