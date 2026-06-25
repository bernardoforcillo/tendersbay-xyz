import { describe, expect, it } from 'vitest';

const modules = import.meta.glob('./*/common.json', { eager: true }) as Record<
  string,
  { default: { landing: { search?: Record<string, string> } } }
>;

const entries = Object.entries(modules);
const REQUIRED = ['placeholder', 'badge', 'hint', 'ariaLabel'] as const;

describe('landing.search locale keys', () => {
  it('covers all 24 locales', () => {
    expect(entries).toHaveLength(24);
  });

  it.each(entries)('%s defines every search key, non-empty', (_path, mod) => {
    const search = mod.default.landing.search;
    expect(search).toBeDefined();
    for (const key of REQUIRED) {
      expect(search?.[key], key).toBeTruthy();
    }
  });
});
