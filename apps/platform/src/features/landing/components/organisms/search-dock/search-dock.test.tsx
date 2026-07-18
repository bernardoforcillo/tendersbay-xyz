import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { LANDING_CARRY_OVER_KEY } from '~/lib/landing-carry-over';
import { renderWithI18n } from '~/test/utils';

const useLandingSearchMock = vi.fn(
  (..._args: unknown[]): { status: string; results: unknown[] } => ({
    status: 'idle',
    results: [],
  }),
);
vi.mock('./use-landing-search', () => ({
  useLandingSearch: (...args: unknown[]) => useLandingSearchMock(...args),
}));

// Stub the results view — the dock's own job is to render the input and forward
// the hook's state upward; SearchResults renders the cards and has its own concern.
vi.mock('./search-results', () => ({
  SearchResults: ({ status, results }: { status: string; results: unknown[] }) => (
    <div data-testid="search-results" data-status={status} data-count={results.length} />
  ),
}));

import { SearchDock } from './index';

describe('SearchDock', () => {
  beforeEach(() => {
    useLandingSearchMock.mockReturnValue({ status: 'idle', results: [] });
    sessionStorage.clear();
  });

  it('renders a labelled, enabled search input with the first localized example as placeholder', () => {
    renderWithI18n(<SearchDock />, 'en-ie');
    const input = screen.getByRole('searchbox', { name: 'Search' });
    expect(input).toBeEnabled();
    expect(input).toHaveAttribute('placeholder', 'Public school renovations');
  });

  it('localizes the example placeholder (it-it)', () => {
    renderWithI18n(<SearchDock />, 'it-it');
    expect(screen.getByRole('searchbox')).toHaveAttribute(
      'placeholder',
      'Ristrutturazione di scuole pubbliche',
    );
  });

  it('drives the search hook from the typed query', async () => {
    const user = userEvent.setup();
    renderWithI18n(<SearchDock />, 'en-ie');
    await user.type(screen.getByRole('searchbox', { name: 'Search' }), 'roads');
    expect(useLandingSearchMock).toHaveBeenLastCalledWith('roads', expect.any(Object));
  });

  it('forwards the hook status and results to the results view', () => {
    useLandingSearchMock.mockReturnValue({
      status: 'results',
      results: [{ id: 'a' }, { id: 'b' }],
    });
    renderWithI18n(<SearchDock />, 'en-ie');
    const results = screen.getByTestId('search-results');
    expect(results).toHaveAttribute('data-status', 'results');
    expect(results).toHaveAttribute('data-count', '2');
  });

  it('no longer renders the old disabled teaser button or filter chips', () => {
    renderWithI18n(<SearchDock />, 'en-ie');
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
    for (const label of ['Country', 'Sector', 'Deadline', 'Value']) {
      expect(screen.queryByRole('button', { name: label })).not.toBeInTheDocument();
    }
  });

  it('persists the resolved query (and empty filters) to the landing carry-over', async () => {
    type Options = { onResolved?: (info: { queryLength: number; resultCount: number }) => void };
    useLandingSearchMock.mockImplementation((...args: unknown[]) => {
      const [query, options] = args as [string, Options | undefined];
      const trimmed = query.trim();
      if (trimmed.length >= 2) {
        options?.onResolved?.({ queryLength: trimmed.length, resultCount: 1 });
      }
      return { status: 'idle', results: [] };
    });
    const user = userEvent.setup();
    renderWithI18n(<SearchDock />, 'en-ie');

    await user.type(screen.getByRole('searchbox', { name: 'Search' }), 'roads');

    expect(JSON.parse(sessionStorage.getItem(LANDING_CARRY_OVER_KEY) ?? 'null')).toEqual({
      query: 'roads',
      filters: {},
    });
  });
});
