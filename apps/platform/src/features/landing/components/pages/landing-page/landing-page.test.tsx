import { screen, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { renderWithI18n } from '~/test/utils';

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => vi.fn(),
  Link: ({ to, children }: { to: string; children?: ReactNode }) => <a href={to}>{children}</a>,
}));

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
      expect(document.title).toBe('tendersbay — EU public tenders, awarded by your AI agents');
    });
  }, 20000);

  it('syncs the existing description and social meta tags without duplicating them', async () => {
    // Seed the static tags the seo plugin injects at build time.
    document.head.innerHTML = [
      '<meta name="description" content="static">',
      '<meta property="og:title" content="static">',
      '<meta property="og:description" content="static">',
      '<meta name="twitter:title" content="static">',
      '<meta name="twitter:description" content="static">',
    ].join('');

    renderWithI18n(<LandingPage />, 'en-ie');

    const description =
      'AI agents that hunt down the right public tenders across Europe, demolish the paperwork, and walk your SME from the first tender opportunity all the way to the award.';
    const title = 'tendersbay — EU public tenders, awarded by your AI agents';
    await waitFor(() => {
      expect(document.head.querySelector('meta[name="description"]')).toHaveAttribute(
        'content',
        description,
      );
    });
    expect(document.head.querySelector('meta[property="og:title"]')).toHaveAttribute(
      'content',
      title,
    );
    expect(document.head.querySelector('meta[property="og:description"]')).toHaveAttribute(
      'content',
      description,
    );
    expect(document.head.querySelector('meta[name="twitter:title"]')).toHaveAttribute(
      'content',
      title,
    );
    expect(document.head.querySelector('meta[name="twitter:description"]')).toHaveAttribute(
      'content',
      description,
    );
    expect(document.head.querySelectorAll('meta[name="description"]')).toHaveLength(1);
  }, 20000);
});
