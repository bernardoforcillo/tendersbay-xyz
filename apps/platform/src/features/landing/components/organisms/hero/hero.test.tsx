import { screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { renderWithI18n } from '~/test/utils';

// The hero deck fetches on mount; with no backend in tests it falls back to the
// curated pool, so the deck (and its honest "Sample results" label) still render.
vi.mock('~/lib/api/client', () => ({
  tenderClient: { searchTenders: vi.fn().mockRejectedValue(new Error('no backend in tests')) },
}));

import { Hero } from './index';

describe('Hero', () => {
  it('renders the headline, both CTAs and the trust line', async () => {
    renderWithI18n(<Hero />, 'en-ie');
    // findBy flushes the sample-tender loader microtask inside act.
    await screen.findByRole('heading', { level: 1 });
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(
      'The tender they already counted as theirs?',
    );
    expect(screen.getByRole('link', { name: /put your agents to work/i })).toHaveAttribute(
      'href',
      '#agents',
    );
    expect(screen.getByRole('link', { name: /see the vision/i })).toHaveAttribute(
      'href',
      '#vision',
    );
    expect(screen.getByText('27 countries, one search')).toBeInTheDocument();
  });

  it('labels the sample deck honestly (no live/real-time claim)', async () => {
    renderWithI18n(<Hero />, 'en-ie');
    expect(await screen.findByText('Sample results')).toBeInTheDocument();
  });
});
