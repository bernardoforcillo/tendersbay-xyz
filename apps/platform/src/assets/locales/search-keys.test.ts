import { describe, expect, it } from 'vitest';

type Search = {
  label?: string;
  hint?: string;
  examples?: string[];
  filters?: { country?: string; sector?: string; deadline?: string; value?: string };
};

const modules = import.meta.glob('./*/common.json', { eager: true }) as Record<
  string,
  { default: { landing: { search?: Search } } }
>;

const entries = Object.entries(modules);

const FILTER_KEYS = ['country', 'sector', 'deadline', 'value'] as const;

describe('landing.search locale keys', () => {
  it('covers all 24 locales', () => {
    expect(entries).toHaveLength(24);
  });

  it.each(entries)('%s defines label, hint and at least 8 examples', (_path, mod) => {
    const search = mod.default.landing.search;
    expect(search?.label, 'label').toBeTruthy();
    expect(search?.hint, 'hint').toBeTruthy();
    expect(Array.isArray(search?.examples), 'examples is array').toBe(true);
    expect((search?.examples ?? []).length, 'at least 8 examples').toBeGreaterThanOrEqual(8);
    expect(
      (search?.examples ?? []).every((e) => typeof e === 'string' && e.trim().length > 0),
      'every example is a non-empty string',
    ).toBe(true);
  });

  it.each(entries)('%s carries all four filter labels', (_path, mod) => {
    const filters = mod.default.landing.search?.filters;
    for (const key of FILTER_KEYS) {
      expect(filters?.[key], `filters.${key}`).toBeTruthy();
    }
  });
});
