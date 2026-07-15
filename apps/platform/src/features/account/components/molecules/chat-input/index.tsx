import { ArrowUp } from 'lucide-react';
import { type FormEvent, useState } from 'react';
import { Button, Input, TextField } from 'react-aria-components';
import { useTranslation } from 'react-i18next';

type ChatInputProps = {
  onSend: (message: string) => void;
  disabled?: boolean;
  placeholder?: string;
};

export function ChatInput({ onSend, disabled, placeholder }: ChatInputProps) {
  const { t } = useTranslation();
  const [value, setValue] = useState('');

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    const trimmed = value.trim();
    if (!trimmed || disabled) return;
    onSend(trimmed);
    setValue('');
  }

  return (
    <form onSubmit={handleSubmit} className="flex items-end gap-2">
      <TextField
        value={value}
        onChange={setValue}
        aria-label={t('account.explore.chatInputLabel', { defaultValue: 'Message' })}
        className="flex-1"
      >
        <Input
          disabled={disabled}
          placeholder={
            placeholder ??
            t('account.explore.chatPlaceholder', { defaultValue: 'Ask anything about tenders…' })
          }
          className="w-full rounded-2xl border border-ink-200 bg-white px-4 py-3 text-sm outline-none placeholder:text-ink-400 disabled:opacity-50 data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-500"
        />
      </TextField>
      <Button
        type="submit"
        isDisabled={disabled || !value.trim()}
        className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-brand-600 text-white outline-none transition-colors hover:bg-brand-700 disabled:opacity-40 data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-500"
      >
        <ArrowUp size={18} />
      </Button>
    </form>
  );
}
