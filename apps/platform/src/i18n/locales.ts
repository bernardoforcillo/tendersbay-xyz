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

export function isSupportedLocale(value: string): value is Locale {
  return (SUPPORTED_LOCALES as readonly string[]).includes(value);
}

export function matchLanguageTag(tag: string): Locale | undefined {
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
