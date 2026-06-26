import { describe, expect, it } from 'vitest';
import { i18n } from '~/i18n';

describe('i18n', () => {
  it('initializes synchronously with the default locale', () => {
    expect(i18n.language).toBe('en-ie');
  });

  it('resolves a bundled translation key from the common namespace', () => {
    const resolved = i18n.t('landing.meta.title');
    expect(resolved).not.toBe('landing.meta.title');
    expect(resolved.length).toBeGreaterThan(0);
  });
});
