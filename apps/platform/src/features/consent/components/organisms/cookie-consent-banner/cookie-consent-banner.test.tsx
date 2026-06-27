import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { I18nextProvider } from 'react-i18next';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ConsentProvider } from '~/features/consent';
import { i18n } from '~/i18n';
import { CookieConsentBanner } from './index';

const stub = { opt_in_capturing: vi.fn(), opt_out_capturing: vi.fn() };

// Force analytics "active" so the banner's gate passes, without real PostHog.
vi.mock('~/analytics/posthog', () => ({
  initAnalytics: () => stub,
  getAnalytics: () => stub,
}));

function renderBanner() {
  return render(
    <I18nextProvider i18n={i18n}>
      <ConsentProvider>
        <CookieConsentBanner />
      </ConsentProvider>
    </I18nextProvider>,
  );
}

beforeEach(() => {
  localStorage.clear();
  stub.opt_in_capturing.mockClear();
  stub.opt_out_capturing.mockClear();
});

describe('CookieConsentBanner', () => {
  it('shows when no choice is stored and opts in on accept', async () => {
    renderBanner();
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    await userEvent.click(screen.getByText(i18n.t('consent.accept')));
    expect(stub.opt_in_capturing).toHaveBeenCalledTimes(1);
    expect(localStorage.getItem('tb_consent')).toBe('granted');
  });

  it('opts out on decline', async () => {
    renderBanner();
    await userEvent.click(screen.getByText(i18n.t('consent.reject')));
    expect(stub.opt_out_capturing).toHaveBeenCalledTimes(1);
    expect(localStorage.getItem('tb_consent')).toBe('denied');
  });

  it('stays hidden once a choice is stored', () => {
    localStorage.setItem('tb_consent', 'denied');
    renderBanner();
    expect(screen.queryByRole('dialog')).toBeNull();
  });
});
