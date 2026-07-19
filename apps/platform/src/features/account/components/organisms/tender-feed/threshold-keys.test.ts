import { describe, expect, it } from 'vitest';

// Completeness guard for the EU-threshold badge copy: every one of the 24 bundled locales must
// define both `tenders.threshold` labels, so a locale that missed the batch splice fails loudly
// here rather than shipping the raw i18n key to a user. Mirrors the `tenders-keys.test.ts`
// precedent (relative `common.json` glob — the app's established test pattern).
type LocaleModule = { default: Record<string, unknown> };

const modules = import.meta.glob('../../../../../assets/locales/*/common.json', {
  eager: true,
}) as Record<string, LocaleModule>;
const entries = Object.entries(modules);

function get(obj: unknown, path: string): unknown {
  return path
    .split('.')
    .reduce<unknown>((acc, key) => (acc as Record<string, unknown> | undefined)?.[key], obj);
}

const REQUIRED_KEYS = ['tenders.threshold.belowEu', 'tenders.threshold.aboveEu'] as const;

describe('tenders.threshold locale keys', () => {
  it('covers all 24 locales', () => {
    expect(entries).toHaveLength(24);
  });

  it.each(entries)('%s defines every threshold key', (_path, mod) => {
    for (const key of REQUIRED_KEYS) {
      expect(get(mod.default, key), key).toBeTruthy();
    }
  });
});
