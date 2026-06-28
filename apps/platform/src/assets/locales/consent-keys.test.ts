import { describe, expect, it } from 'vitest';

type Consent = {
  title?: string;
  body?: string;
  accept?: string;
  reject?: string;
};

const modules = import.meta.glob('./*/common.json', { eager: true }) as Record<
  string,
  { default: { consent?: Consent } }
>;

const entries = Object.entries(modules);

describe('consent locale keys', () => {
  it('covers all 24 locales', () => {
    expect(entries).toHaveLength(24);
  });

  it.each(entries)('%s defines consent.title, body, accept and reject', (_path, mod) => {
    const consent = mod.default.consent;
    expect(consent?.title, 'consent.title').toBeTruthy();
    expect(consent?.body, 'consent.body').toBeTruthy();
    expect(consent?.accept, 'consent.accept').toBeTruthy();
    expect(consent?.reject, 'consent.reject').toBeTruthy();
  });
});
