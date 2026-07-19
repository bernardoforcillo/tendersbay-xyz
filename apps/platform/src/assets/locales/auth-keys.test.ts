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
  'auth.login.title',
  'auth.login.description',
  'auth.login.email',
  'auth.login.password',
  'auth.login.forgotPassword',
  'auth.login.submitting',
  'auth.login.submit',
  'auth.login.noAccount',
  'auth.login.signUp',
  'auth.signup.checkEmail',
  'auth.signup.verifyPrompt',
  'auth.signup.checkEmailHint',
  'auth.signup.backToLogin',
  'auth.signup.title',
  'auth.signup.description',
  'auth.signup.displayName',
  'auth.signup.email',
  'auth.signup.password',
  'auth.signup.passwordHint',
  'auth.signup.submit',
  'auth.signup.login',
  'auth.signup.signIn',
  'auth.signup.validation.nameMin',
  'auth.signup.validation.emailInvalid',
  'auth.signup.validation.passwordMin',
  'auth.forgot.checkEmail',
  'auth.forgot.checkEmailBody',
  'auth.forgot.checkEmailHint',
  'auth.forgot.backToLogin',
  'auth.forgot.title',
  'auth.forgot.description',
  'auth.forgot.email',
  'auth.forgot.submitting',
  'auth.forgot.submit',
  'auth.reset.invalidTitle',
  'auth.reset.invalid',
  'auth.reset.requestNew',
  'auth.reset.title',
  'auth.reset.description',
  'auth.reset.password',
  'auth.reset.submitting',
  'auth.reset.submit',
  'auth.verify.errorTitle',
  'auth.verify.error',
  'auth.verify.successTitle',
  'auth.verify.success',
  'auth.verify.loadingTitle',
  'auth.verify.loading',
  'landing.header.login',
  'landing.header.signup',
] as const;

describe('auth locale keys', () => {
  it('covers all 24 locales', () => {
    expect(entries).toHaveLength(24);
  });

  it.each(entries)('%s defines every auth key', (_path, mod) => {
    for (const key of REQUIRED_KEYS) {
      expect(get(mod.default, key), key).toBeTruthy();
    }
  });
});
