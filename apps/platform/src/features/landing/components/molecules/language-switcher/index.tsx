import { useNavigate } from '@tanstack/react-router';
import type { Key } from 'react-aria-components';
import { Button, ListBox, ListBoxItem, Popover, Select, SelectValue } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { Icon } from '~/features/landing/components/atoms';
import { isSupportedLocale, SUPPORTED_LOCALES, writeLocaleCookie } from '~/i18n/detect-locale';

function nativeName(locale: string): string {
  const [language] = locale.split('-');
  const display = new Intl.DisplayNames([locale], { type: 'language' });
  const label = display.of(language ?? locale) ?? locale;
  return label.charAt(0).toUpperCase() + label.slice(1);
}

export function LanguageSwitcher() {
  const { i18n } = useTranslation();
  const navigate = useNavigate();

  function onChange(key: Key | null) {
    if (key === null) return;
    const next = String(key);
    if (!isSupportedLocale(next)) {
      return;
    }
    writeLocaleCookie(next);
    void navigate({ to: '/$locale', params: { locale: next } });
  }

  return (
    <Select aria-label="Language" selectedKey={i18n.language} onSelectionChange={onChange}>
      <Button className="inline-flex items-center gap-2 rounded-full border border-brand-200 bg-brand-50 px-3 py-1.5 text-sm font-semibold text-brand-700 outline-none data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600">
        <SelectValue />
        <Icon name="chevron-down" className="text-[14px]" />
      </Button>
      <Popover className="max-h-72 w-48 overflow-auto rounded-xl border border-cream-300 bg-white p-1 shadow-xl">
        <ListBox className="outline-none">
          {SUPPORTED_LOCALES.map((locale) => (
            <ListBoxItem
              key={locale}
              id={locale}
              textValue={nativeName(locale)}
              className="cursor-pointer rounded-lg px-3 py-2 text-sm text-ink-800 outline-none data-[focus]:bg-brand-50 data-[selected]:font-bold data-[selected]:text-brand-700"
            >
              {nativeName(locale)}
            </ListBoxItem>
          ))}
        </ListBox>
      </Popover>
    </Select>
  );
}
