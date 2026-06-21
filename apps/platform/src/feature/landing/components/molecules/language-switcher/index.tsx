import { useNavigate } from '@tanstack/react-router';
import type { ChangeEvent } from 'react';
import { useTranslation } from 'react-i18next';
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

  function onChange(event: ChangeEvent<HTMLSelectElement>) {
    const next = event.target.value;
    if (!isSupportedLocale(next)) {
      return;
    }
    writeLocaleCookie(next);
    void navigate({ to: '/$locale/', params: { locale: next } });
  }

  return (
    <select
      aria-label="Language"
      value={i18n.language}
      onChange={onChange}
      className="rounded border border-slate-300 bg-white px-2 py-1 text-slate-900"
    >
      {SUPPORTED_LOCALES.map((locale) => (
        <option key={locale} value={locale}>
          {nativeName(locale)}
        </option>
      ))}
    </select>
  );
}
