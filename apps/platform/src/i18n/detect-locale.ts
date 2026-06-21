export const SUPPORTED_LOCALES = [
  'bg-bg',
  'hr-hr',
  'cs-cz',
  'da-dk',
  'nl-nl',
  'en-ie',
  'et-ee',
  'fi-fi',
  'fr-fr',
  'de-de',
  'el-gr',
  'hu-hu',
  'ga-ie',
  'it-it',
  'lv-lv',
  'lt-lt',
  'mt-mt',
  'pl-pl',
  'pt-pt',
  'ro-ro',
  'sk-sk',
  'sl-si',
  'es-es',
  'sv-se',
] as const;

export type Locale = (typeof SUPPORTED_LOCALES)[number];

export const DEFAULT_LOCALE: Locale = 'en-ie';

const LOCALE_COOKIE = 'locale';

export function isSupportedLocale(value: string): value is Locale {
  return (SUPPORTED_LOCALES as readonly string[]).includes(value);
}

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

function matchLanguageTag(tag: string): Locale | undefined {
  const normalized = tag.toLowerCase();
  if (isSupportedLocale(normalized)) {
    return normalized;
  }
  const [language] = normalized.split('-');
  if (!language) {
    return undefined;
  }
  return SUPPORTED_LOCALES.find((locale) => locale.split('-')[0] === language);
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
