import { describe, expect, it } from 'vitest';
import { i18n } from '~/i18n';

describe('landing copy', () => {
  it('resolves the hero headline in en-ie', async () => {
    await i18n.changeLanguage('en-ie');
    expect(i18n.t('landing.hero.titleLead')).toBe('Win your next tender');
  });

  it('resolves the hero headline in it-it', async () => {
    await i18n.changeLanguage('it-it');
    expect(i18n.t('landing.hero.titleLead')).toBe('Vinci la tua prossima gara');
  });

  it('exposes problem items as an array of three', async () => {
    await i18n.changeLanguage('en-ie');
    const items = i18n.t('landing.problem.items', { returnObjects: true }) as unknown[];
    expect(items).toHaveLength(3);
  });
});
