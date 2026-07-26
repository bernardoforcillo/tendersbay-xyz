import { render } from '@testing-library/react';
import type { ReactNode } from 'react';
import { I18nextProvider } from 'react-i18next';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { i18n } from '~/i18n';

const { capture, search } = vi.hoisted(() => ({
  capture: vi.fn(),
  search: { current: {} as { entry?: string } },
}));
vi.mock('posthog-js/react', () => ({ usePostHog: () => ({ capture }) }));
vi.mock('@tanstack/react-router', () => ({
  useParams: () => ({ locale: 'en-ie' }),
  useSearch: () => search.current,
  Link: ({ to, children }: { to: string; children?: ReactNode }) => <a href={to}>{children}</a>,
}));
vi.mock('~/lib/redirect', () => ({ useRedirectParam: () => ({ raw: undefined }) }));
vi.mock('~/lib/api/client', () => ({ authClient: { signUp: vi.fn() } }));

import { SignupPage } from './index';

const renderPage = () => {
  void i18n.changeLanguage('en-ie');
  return render(
    <I18nextProvider i18n={i18n}>
      <SignupPage />
    </I18nextProvider>,
  );
};

describe('SignupPage analytics', () => {
  afterEach(() => {
    capture.mockClear();
  });

  it('fires signup_started with the entry from search', () => {
    search.current = { entry: 'hero' };
    renderPage();
    expect(capture).toHaveBeenCalledTimes(1);
    expect(capture).toHaveBeenCalledWith('signup_started', { entry: 'hero' });
  });

  it("defaults entry to 'direct' when absent", () => {
    search.current = {};
    renderPage();
    expect(capture).toHaveBeenCalledWith('signup_started', { entry: 'direct' });
  });
});
