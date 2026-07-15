import { describe, expect, it } from 'vitest';

// landing.hero titleLead/titleHighlight/subtitle are build-critical: vite.config.ts
// reads them from every locale file at config time to build the per-locale
// <noscript> hero content block emitted into dist/<locale>/index.html.
type Landing = {
  hero?: { titleLead?: string; titleHighlight?: string; subtitle?: string };
};

const modules = import.meta.glob('./*/common.json', { eager: true }) as Record<
  string,
  { default: { landing: Landing } }
>;

const entries = Object.entries(modules);

describe('landing hero locale keys', () => {
  it('covers all 24 locales', () => {
    expect(entries).toHaveLength(24);
  });

  it.each(
    entries,
  )('%s has a non-empty landing.hero lead, highlight, and subtitle', (_path, mod) => {
    const hero = mod.default.landing.hero;
    expect(hero?.titleLead, 'landing.hero.titleLead').toBeTruthy();
    expect(hero?.titleHighlight, 'landing.hero.titleHighlight').toBeTruthy();
    expect(hero?.subtitle, 'landing.hero.subtitle').toBeTruthy();
  });
});
