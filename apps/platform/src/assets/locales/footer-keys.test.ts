import { describe, expect, it } from 'vitest';

type Column = { heading?: string; links?: string[] };
const modules = import.meta.glob('./*/common.json', { eager: true }) as Record<
  string,
  {
    default: {
      landing: {
        footer: { social?: string; columns?: Column[] };
        cta?: { eyebrow?: string; title?: string; body?: string; button?: string };
      };
    };
  }
>;

const entries = Object.entries(modules);

describe('landing footer + cta locale keys', () => {
  it('covers all 24 locales', () => {
    expect(entries).toHaveLength(24);
  });

  it.each(entries)('%s defines footer.social and 3 non-empty columns', (_path, mod) => {
    const footer = mod.default.landing.footer;
    expect(footer.social, 'social').toBeTruthy();
    expect(Array.isArray(footer.columns), 'columns is array').toBe(true);
    expect(footer.columns, 'three columns').toHaveLength(3);
    for (const col of footer.columns ?? []) {
      expect(col.heading, 'column heading').toBeTruthy();
      expect(Array.isArray(col.links), 'column links is array').toBe(true);
      expect((col.links ?? []).length, 'column links non-empty').toBeGreaterThan(0);
    }
  });

  it.each(entries)('%s defines cta eyebrow, title, body and button', (_path, mod) => {
    const cta = mod.default.landing.cta;
    expect(cta?.eyebrow, 'eyebrow').toBeTruthy();
    expect(cta?.title, 'title').toBeTruthy();
    expect(cta?.body, 'body').toBeTruthy();
    expect(cta?.button, 'button').toBeTruthy();
  });
});
