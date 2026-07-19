import type { TenderResult } from '@tendersbay/proto/tender/v1/tender_pb';
import { screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { renderWithI18n } from '~/test/utils';

vi.mock('~/features/tenders', () => ({
  useTenderLink: () => (id: string, children: ReactNode, className?: string) => (
    <a href={`/tenders/${id}`} className={className}>
      {children}
    </a>
  ),
}));

import { SearchResults } from './search-results';

function fixture(overrides: Partial<TenderResult> = {}): TenderResult {
  return {
    $typeName: 'tender.v1.TenderResult',
    id: 't-1',
    title: 'Supply of road maintenance services',
    buyerName: 'City of Lisbon',
    status: 'open',
    procedureType: 'open',
    country: 'PT',
    cpv: '45233141',
    value: 240_000n,
    currency: 'EUR',
    publishedAt: '',
    deadline: '',
    relevanceScore: 0,
    source: 'ted',
    sourceRef: 'ref-1',
    sourceUrl: '',
    ...overrides,
  } as TenderResult;
}

describe('SearchResults', () => {
  it('links each result card to its tender detail page', () => {
    renderWithI18n(
      <SearchResults
        status="results"
        results={[fixture({ id: 't-1' }), fixture({ id: 't-2', title: 'Repair of city bridges' })]}
      />,
    );
    const links = screen.getAllByRole('link');
    expect(links).toHaveLength(2);
    expect(links[0]).toHaveAttribute('href', '/tenders/t-1');
    expect(links[1]).toHaveAttribute('href', '/tenders/t-2');
  });

  it.each(['empty', 'loading', 'error'] as const)('renders no links in the %s state', (status) => {
    renderWithI18n(<SearchResults status={status} results={[]} />);
    expect(screen.queryByRole('link')).not.toBeInTheDocument();
  });
});
