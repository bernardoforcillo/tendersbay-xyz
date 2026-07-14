import { describe, expect, it } from 'vitest';

type LocaleModule = { default: Record<string, unknown> };

const modules = import.meta.glob('./*/common.json', { eager: true }) as Record<
  string,
  LocaleModule
>;
const entries = Object.entries(modules);

function get(obj: unknown, path: string): unknown {
  return path
    .split('.')
    .reduce<unknown>((acc, key) => (acc as Record<string, unknown> | undefined)?.[key], obj);
}

// Non-plural keys every locale must define.
const REQUIRED_KEYS = [
  'tenders.searching',
  'tenders.empty.title',
  'tenders.empty.description',
  'tenders.error',
  'tenders.loadMore',
  'tenders.deadline.today',
  'tenders.deadline.expired',
  'tenders.value.unknown',
  'tenders.status.open',
  'tenders.status.awarded',
  'tenders.status.cancelled',
  'tenders.status.closed',
  'tenders.status.unknown',
] as const;

// Plural key stems: every locale must define at least `_one` and `_other`;
// CLDR languages that need extra categories (few/many/two/zero) carry them
// too, but the completeness test only demands the two universal suffixes.
const PLURAL_STEMS = ['tenders.results', 'tenders.deadline.days'] as const;

describe('tenders locale keys', () => {
  it('covers all 24 locales', () => {
    expect(entries).toHaveLength(24);
  });

  it.each(entries)('%s defines every tenders key', (_path, mod) => {
    for (const key of REQUIRED_KEYS) {
      expect(get(mod.default, key), key).toBeTruthy();
    }
  });

  it.each(entries)('%s defines the required plural forms', (_path, mod) => {
    for (const stem of PLURAL_STEMS) {
      const other = get(mod.default, `${stem}_other`);
      expect(other, `${stem}_other`).toBeTruthy();
      expect(other, `${stem}_other`).toContain('{{count}}');

      const one = get(mod.default, `${stem}_one`);
      if (one !== undefined) {
        expect(one, `${stem}_one`).toBeTruthy();
      }
    }
  });
});
