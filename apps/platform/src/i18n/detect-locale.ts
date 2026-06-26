import { DEFAULT_LOCALE, isSupportedLocale, type Locale, matchLanguageTag } from './locales';

export type { Locale } from './locales';
export { DEFAULT_LOCALE, isSupportedLocale, SUPPORTED_LOCALES } from './locales';

const LOCALE_COOKIE = 'locale';

function readCookie(name: string): string | undefined {
  const prefix = `${name}=`;
  const entry = document.cookie.split('; ').find((row) => row.startsWith(prefix));
  return entry?.slice(prefix.length);
}

export function readLocaleCookie(): Locale | undefined {
  const value = readCookie(LOCALE_COOKIE);
  return value && isSupportedLocale(value) ? value : undefined;
}

export function writeLocaleCookie(locale: Locale): void {
  document.cookie = `${LOCALE_COOKIE}=${locale}; path=/; max-age=31536000; SameSite=Lax`;
}

export function detectLocale(): Locale {
  const fromCookie = readLocaleCookie();
  if (fromCookie) {
    return fromCookie;
  }
  const candidates = navigator.languages?.length ? navigator.languages : [navigator.language];
  for (const tag of candidates) {
    if (!tag) {
      continue;
    }
    const match = matchLanguageTag(tag);
    if (match) {
      return match;
    }
  }
  return DEFAULT_LOCALE;
}
