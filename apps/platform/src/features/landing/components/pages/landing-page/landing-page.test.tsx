import { screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { renderWithI18n } from '~/test/utils';

vi.mock('@tanstack/react-router', () => ({ useNavigate: () => vi.fn() }));

import { LandingPage } from './index';

describe('LandingPage', () => {
  // This is the only test that renders the full template (incl. the 27-flag
  // coverage grid through motion); the synchronous jsdom render can exceed the
  // default 5s timeout under full-suite parallelism, so give it extra headroom.
  it('renders the landing template and sets the document title', async () => {
    renderWithI18n(<LandingPage />, 'en-ie');
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(
      'Your next European tender?',
    );
    expect(screen.getByRole('heading', { name: /27 EU countries/i })).toBeInTheDocument();
    await waitFor(() => {
      expect(document.title).toBe('tendersbay — Your next European tender, awarded');
    });
  }, 20000);
});
