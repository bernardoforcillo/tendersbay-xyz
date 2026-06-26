import { describe, expect, it } from 'vitest';

const modules = import.meta.glob('./*/common.json', { eager: true }) as Record<
  string,
  { default: { landing: { search?: { label?: string; hint?: string; examples?: string[] } } } }
>;

const entries = Object.entries(modules);

describe('landing.search locale keys', () => {
  it('covers all 24 locales', () => {
    expect(entries).toHaveLength(24);
  });

  it.each(entries)('%s defines label, hint and non-empty examples', (_path, mod) => {
    const search = mod.default.landing.search;
    expect(search?.label, 'label').toBeTruthy();
    expect(search?.hint, 'hint').toBeTruthy();
    expect(Array.isArray(search?.examples), 'examples is array').toBe(true);
    expect((search?.examples ?? []).length, 'examples non-empty').toBeGreaterThan(0);
  });
});
