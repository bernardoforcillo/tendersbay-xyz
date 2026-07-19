import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { renderWithI18n } from '~/test/utils';

const getCoverage = vi.fn();
vi.mock('~/lib/api/client', () => ({
  tenderClient: {
    getCoverage: (...args: unknown[]) => getCoverage(...args),
  },
}));

const { capture } = vi.hoisted(() => ({ capture: vi.fn() }));
vi.mock('posthog-js/react', () => ({ usePostHog: () => ({ capture }) }));

import { CoverageSection } from './index';

beforeEach(() => {
  getCoverage.mockReset();
  capture.mockReset();
  // Default: the backend reports live coverage for Italy only.
  getCoverage.mockResolvedValue({ countries: ['IT'] });
  // Force the reduced-motion (static grid) variant so assertions target a
  // deterministic 27-tile layout instead of the marquee's duplicated tracks.
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches: query.includes('reduce'),
    media: query,
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }));
});

describe('CoverageSection', () => {
  it('renders all 27 EU countries as flag buttons', async () => {
    renderWithI18n(<CoverageSection />, 'en-ie');
    expect(screen.getAllByRole('button')).toHaveLength(27);
    // Let the coverage fetch settle so its state update stays inside act().
    await waitFor(() => expect(getCoverage).toHaveBeenCalled());
  });

  it('fires coverage_market_focused with categorical props on flag focus', async () => {
    const user = userEvent.setup();
    renderWithI18n(<CoverageSection />, 'en-ie');
    await waitFor(() => expect(getCoverage).toHaveBeenCalled());
    await user.click(await screen.findByRole('button', { name: /italy/i }));
    expect(capture).toHaveBeenCalledWith('coverage_market_focused', {
      country: 'IT',
      status: 'available',
      location: 'coverage_section',
    });
  });

  it('lights the flag for a covered country and leaves the rest coming-soon', async () => {
    renderWithI18n(<CoverageSection />, 'en-ie');
    // Italy flips to "Live" once GetCoverage resolves with ['IT'].
    expect(await screen.findByRole('button', { name: /Italy.*Live/i })).toBeInTheDocument();
    // A country not in the coverage set stays coming-soon.
    expect(screen.getByRole('button', { name: /Poland.*Coming soon/i })).toBeInTheDocument();
  });

  it('localizes country names via Intl.DisplayNames', async () => {
    renderWithI18n(<CoverageSection />, 'it-it');
    expect(screen.getByRole('button', { name: /Italia/ })).toBeInTheDocument();
    await waitFor(() => expect(getCoverage).toHaveBeenCalled());
  });

  it('reveals a card with the national portal when a flag is focused', async () => {
    const user = userEvent.setup();
    renderWithI18n(<CoverageSection />, 'en-ie');
    await user.tab();
    const card = await screen.findByRole('tooltip');
    expect(card).toHaveTextContent('BBG');
  });
});
