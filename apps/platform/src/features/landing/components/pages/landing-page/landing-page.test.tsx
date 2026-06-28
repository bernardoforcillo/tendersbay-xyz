import { screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { renderWithI18n } from '~/test/utils';

vi.mock('@tanstack/react-router', () => ({ useNavigate: () => vi.fn() }));

import { LandingPage } from './index';

beforeEach(() => {
  // Render the coverage section's reduced-motion (static grid) variant so the
  // full-template render stays light and deterministic (no marquee tracks).
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

describe('LandingPage', () => {
  it('renders the landing template and sets the document title', async () => {
    renderWithI18n(<LandingPage />, 'en-ie');
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(
      'The tender they already counted as theirs?',
    );
    expect(screen.getByRole('heading', { name: /taking them one by one/i })).toBeInTheDocument();
    await waitFor(() => {
      expect(document.title).toBe('tendersbay — European tenders, awarded by your AI agents');
    });
  }, 20000);
});
