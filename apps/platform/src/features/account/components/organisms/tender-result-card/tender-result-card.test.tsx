import type { TenderResult } from '@tendersbay/proto/tender/v1/tender_pb';
import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

// Return defaultValue strings (with {{count}} interpolation) without initializing
// the full i18n stack — precedent: chat-window.test.tsx, command-palette.test.tsx.
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, options?: string | { count?: number; defaultValue?: string }) => {
      const opts = typeof options === 'string' ? { defaultValue: options } : options;
      const template = opts?.defaultValue ?? key;
      return typeof opts?.count === 'number'
        ? template.replace('{{count}}', String(opts.count))
        : template;
    },
    i18n: { language: 'en-ie' },
  }),
}));

import { TenderResultCard } from './index';

const ONE_DAY_MS = 86_400_000;

function deadlineAt(daysFromNow: number, now: Date): string {
  return new Date(now.getTime() + daysFromNow * ONE_DAY_MS).toISOString();
}

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
    ...overrides,
  } as TenderResult;
}

describe('TenderResultCard', () => {
  it('renders the title, buyer, mono meta tag, and the value + status line', () => {
    render(<TenderResultCard tender={fixture()} />);

    expect(screen.getByText('Supply of road maintenance services')).toBeInTheDocument();
    expect(screen.getByText('City of Lisbon')).toBeInTheDocument();
    expect(screen.getByText('45233141 · PT')).toBeInTheDocument();
    expect(screen.getByText('€240,000 · open')).toBeInTheDocument();
  });

  it('omits the buyer line when buyerName is empty', () => {
    render(<TenderResultCard tender={fixture({ buyerName: '' })} />);
    expect(screen.queryByText('City of Lisbon')).not.toBeInTheDocument();
  });

  it('shows an urgent pill with the day count 3 days from the deadline', () => {
    const now = new Date();
    render(<TenderResultCard tender={fixture({ deadline: deadlineAt(3, now) })} />);

    const pill = screen.getByText('3 days left');
    expect(pill).toBeInTheDocument();
    expect(pill).toHaveClass('bg-signal-urgent-100');
  });

  it('shows a deadline-tone pill with the day count 10 days from the deadline', () => {
    const now = new Date();
    render(<TenderResultCard tender={fixture({ deadline: deadlineAt(10, now) })} />);

    const pill = screen.getByText('10 days left');
    expect(pill).toBeInTheDocument();
    expect(pill).toHaveClass('bg-signal-warm-100');
  });

  it('shows an urgent "Closed" pill for an expired deadline', () => {
    const now = new Date();
    render(<TenderResultCard tender={fixture({ deadline: deadlineAt(-2, now) })} />);

    const pill = screen.getByText('Closed');
    expect(pill).toBeInTheDocument();
    expect(pill).toHaveClass('bg-signal-urgent-100');
  });

  it('renders no pill when the tender has no deadline', () => {
    render(<TenderResultCard tender={fixture({ deadline: '' })} />);
    expect(screen.queryByText(/day|Closed|Closes today/)).not.toBeInTheDocument();
  });

  it('falls back to "Value undisclosed" when the value is zero', () => {
    render(<TenderResultCard tender={fixture({ value: 0n })} />);
    expect(screen.getByText('Value undisclosed · open')).toBeInTheDocument();
  });
});
