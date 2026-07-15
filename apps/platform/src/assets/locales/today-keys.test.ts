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
  'today.recommended.title',
  'today.recommended.seeAll',
] as const;

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
});
