import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { renderWithI18n } from '~/test/utils';
import { CoverageSection } from './index';

beforeEach(() => {
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
  it('renders all 27 EU countries as flag buttons', () => {
    renderWithI18n(<CoverageSection />, 'en-ie');
    expect(screen.getAllByRole('button')).toHaveLength(27);
  });

  it('marks every country coming-soon in the teaser (none available)', () => {
    renderWithI18n(<CoverageSection />, 'en-ie');
    const buttons = screen.getAllByRole('button');
    expect(buttons.every((b) => /Coming soon/.test(b.getAttribute('aria-label') ?? ''))).toBe(true);
  });

  it('localizes country names via Intl.DisplayNames', () => {
    renderWithI18n(<CoverageSection />, 'it-it');
    expect(screen.getByRole('button', { name: /Italia/ })).toBeInTheDocument();
  });

  it('reveals a card with the national portal when a flag is focused', async () => {
    const user = userEvent.setup();
    renderWithI18n(<CoverageSection />, 'en-ie');
    await user.tab();
    const card = await screen.findByRole('tooltip');
    expect(card).toHaveTextContent('BBG');
  });
});
