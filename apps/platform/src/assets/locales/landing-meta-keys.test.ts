import { describe, expect, it } from 'vitest';

// landing.meta.title/description are build-critical: vite.config.ts reads them
// from every locale file at config time to emit per-locale index.html heads.
type Landing = { meta?: { title?: string; description?: string } };

const modules = import.meta.glob('./*/common.json', { eager: true }) as Record<
  string,
  { default: { landing: Landing } }
>;

const entries = Object.entries(modules);

describe('landing meta locale keys', () => {
  it('covers all 24 locales', () => {
    expect(entries).toHaveLength(24);
  });

  it.each(entries)('%s has a non-empty landing.meta title and description', (_path, mod) => {
    const meta = mod.default.landing.meta;
    expect(meta?.title, 'landing.meta.title').toBeTruthy();
    expect(meta?.description, 'landing.meta.description').toBeTruthy();
  });
});
