import { afterEach, describe, expect, it } from 'vitest';
import {
  DEFAULT_LOCALE,
  detectLocale,
  isSupportedLocale,
  LOCALE_NATIVE_NAMES,
  SUPPORTED_LOCALES,
} from './detect-locale';

function setLanguages(languages: string[]): void {
  Object.defineProperty(navigator, 'languages', { value: languages, configurable: true });
  Object.defineProperty(navigator, 'language', { value: languages[0] ?? '', configurable: true });
}

function clearCookies(): void {
  for (const entry of document.cookie.split(';')) {
    const name = entry.split('=')[0]?.trim();
    if (name) {
      document.cookie = `${name}=; max-age=0; path=/`;
    }
  }
}

afterEach(() => {
  clearCookies();
});

describe('detectLocale', () => {
  it('prefers a valid locale cookie over the browser language', () => {
    document.cookie = 'locale=de-de; path=/';
    setLanguages(['it-IT']);
    expect(detectLocale()).toBe('de-de');
  });

  it('maps a region variant to its supported locale', () => {
    setLanguages(['it-IT']);
    expect(detectLocale()).toBe('it-it');
  });

  it('maps English variants (en-GB, en) to en-ie', () => {
    setLanguages(['en-GB']);
    expect(detectLocale()).toBe('en-ie');
    setLanguages(['en']);
    expect(detectLocale()).toBe('en-ie');
  });

  it('maps a bare language subtag to its locale (de -> de-de)', () => {
    setLanguages(['de']);
    expect(detectLocale()).toBe('de-de');
  });

  it('falls back to the default for a non-EU language', () => {
    setLanguages(['ja-JP']);
    expect(detectLocale()).toBe(DEFAULT_LOCALE);
  });

  it('ignores an invalid cookie and uses the browser language', () => {
    document.cookie = 'locale=xx-xx; path=/';
    setLanguages(['de-DE']);
    expect(detectLocale()).toBe('de-de');
  });
});

describe('isSupportedLocale', () => {
  it('accepts a supported locale and rejects others', () => {
    expect(isSupportedLocale('en-ie')).toBe(true);
    expect(isSupportedLocale('en-gb')).toBe(false);
  });
});

describe('LOCALE_NATIVE_NAMES', () => {
  it('defines a non-empty native name for every supported locale', () => {
    for (const locale of SUPPORTED_LOCALES) {
      const name = LOCALE_NATIVE_NAMES[locale];
      expect(typeof name, locale).toBe('string');
      expect(name.length, locale).toBeGreaterThan(0);
      // Guards against the `Intl.DisplayNames` fallback that renders the bare
      // language code (e.g. Maltese -> "Mt", Irish -> "Ga").
      expect(name.toLowerCase(), locale).not.toBe(locale.split('-')[0]);
    }
  });

  it('uses the correct autonyms for Maltese and Irish', () => {
    expect(LOCALE_NATIVE_NAMES['mt-mt']).toBe('Malti');
    expect(LOCALE_NATIVE_NAMES['ga-ie']).toBe('Gaeilge');
  });
});
