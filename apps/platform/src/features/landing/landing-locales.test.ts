import { describe, expect, it } from 'vitest';
import { i18n } from '~/i18n';
import { SUPPORTED_LOCALES } from '~/i18n/detect-locale';

describe('landing copy completeness', () => {
  it('defines landing.hero.titleLead as a non-empty string in every locale', () => {
    for (const locale of SUPPORTED_LOCALES) {
      const value = i18n.getResource(locale, 'common', 'landing.hero.titleLead');
      expect(typeof value, locale).toBe('string');
      expect((value as string).length, locale).toBeGreaterThan(0);
    }
  });

  it('defines exactly three agent items in every locale', () => {
    for (const locale of SUPPORTED_LOCALES) {
      const items = i18n.getResource(locale, 'common', 'landing.agents.items');
      expect(Array.isArray(items), locale).toBe(true);
      expect((items as unknown[]).length, locale).toBe(3);
    }
  });
});
