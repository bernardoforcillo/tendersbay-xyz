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

const REQUIRED_KEYS = [
  'today.greeting.morning',
  'today.greeting.afternoon',
  'today.greeting.evening',
  'today.greeting.morningNamed',
  'today.greeting.afternoonNamed',
  'today.greeting.eveningNamed',
  'today.resume.title',
  'today.resume.untitled',
  'today.explore.title',
  'today.explore.description',
  'today.explore.action',
  'today.recommended.seeAll',
] as const;

// Plural key stems: every locale must define at least `_one` and `_other`;
// CLDR languages that need extra categories (few/many/two/zero) carry them
// too, but the completeness test only demands the two universal suffixes.
const PLURAL_STEMS = ['today.recommended.clientCount'] as const;

describe('today locale keys', () => {
  it('covers all 24 locales', () => {
    expect(entries).toHaveLength(24);
  });

  it.each(entries)('%s defines every today key', (_path, mod) => {
    for (const key of REQUIRED_KEYS) {
      expect(get(mod.default, key), key).toBeTruthy();
    }
  });

  it.each(entries)('%s keeps the {{name}} placeholder in the named greetings', (_path, mod) => {
    for (const key of [
      'today.greeting.morningNamed',
      'today.greeting.afternoonNamed',
      'today.greeting.eveningNamed',
    ]) {
      expect(get(mod.default, key), key).toContain('{{name}}');
    }
  });

  it.each(entries)('%s defines the required plural forms with both placeholders', (_path, mod) => {
    for (const stem of PLURAL_STEMS) {
      for (const suffix of ['one', 'other'] as const) {
        const form = get(mod.default, `${stem}_${suffix}`);
        expect(form, `${stem}_${suffix}`).toBeTruthy();
        expect(form, `${stem}_${suffix}`).toContain('{{count}}');
        expect(form, `${stem}_${suffix}`).toContain('{{client}}');
      }
      for (const suffix of ['two', 'few', 'many', 'zero'] as const) {
        const form = get(mod.default, `${stem}_${suffix}`);
        if (form !== undefined) {
          expect(form, `${stem}_${suffix}`).toContain('{{count}}');
          expect(form, `${stem}_${suffix}`).toContain('{{client}}');
        }
      }
    }
  });
});
