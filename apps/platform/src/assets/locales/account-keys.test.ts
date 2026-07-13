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
  'account.settings.title',
  'account.profile.title',
  'account.profile.description',
  'account.profile.displayName',
  'account.profile.saved',
  'account.profile.submitting',
  'account.profile.submit',
  'account.changeEmail.checkEmail',
  'account.changeEmail.title',
  'account.changeEmail.prompt',
  'account.changeEmail.description',
  'account.changeEmail.hint',
  'account.changeEmail.newEmail',
  'account.changeEmail.password',
  'account.changeEmail.submitting',
  'account.changeEmail.submit',
  'account.changePassword.successTitle',
  'account.changePassword.title',
  'account.changePassword.successBody',
  'account.changePassword.description',
  'account.changePassword.successHint',
  'account.changePassword.current',
  'account.changePassword.new',
  'account.changePassword.submitting',
  'account.changePassword.submit',
  'account.delete.title',
  'account.delete.description',
  'account.delete.password',
  'account.delete.submitting',
  'account.delete.submit',
] as const;

describe('account locale keys', () => {
  it('covers all 24 locales', () => {
    expect(entries).toHaveLength(24);
  });

  it.each(entries)('%s defines every account key', (_path, mod) => {
    for (const key of REQUIRED_KEYS) {
      expect(get(mod.default, key), key).toBeTruthy();
    }
  });
});
