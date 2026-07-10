import { Button } from 'react-aria-components';
import type { ChatMessage } from '~/store/chat';

type ChoicePromptCardProps = {
  message: ChatMessage;
  isPending: boolean;
  onSubmit: (selectedKey: string, customValue?: string) => void;
};

export function ChoicePromptCard({ message, isPending, onSubmit }: ChoicePromptCardProps) {
  const options = message.choices ?? [];

  return (
    <div className="flex justify-start">
      <div className="max-w-[80%] space-y-2.5 rounded-2xl bg-cream-200 px-4 py-2.5 text-sm leading-relaxed text-ink-900">
        <p>{message.content}</p>
        <div className="flex flex-wrap gap-2">
          {options.map((option) => (
            <Button
              key={option.key}
              isDisabled={!isPending}
              onPress={() => onSubmit(option.key, undefined)}
              className="rounded-full border border-ink-200 bg-white px-3 py-1.5 text-xs font-medium text-ink-900 outline-none transition-colors hover:bg-cream-100 disabled:cursor-default disabled:opacity-50 data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-500"
            >
              {option.key}) {option.label}
            </Button>
          ))}
        </div>
      </div>
    </div>
  );
}
