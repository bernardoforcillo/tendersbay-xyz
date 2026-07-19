import type { TenderResult } from '@tendersbay/proto/tender/v1/tender_pb';
import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { renderWithI18n } from '~/test/utils';
import { TenderResultCard } from './index';

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
    euThreshold: '',
    ...overrides,
  } as TenderResult;
}

describe('TenderResultCard EU-threshold badge', () => {
  it('renders an emphasised below-EU badge when euThreshold=below_eu', () => {
    renderWithI18n(<TenderResultCard tender={fixture({ euThreshold: 'below_eu' })} />);
    const badge = screen.getByTestId('tender-eu-threshold');
    expect(badge).toHaveTextContent(/below/i);
    // "below" (SME-winnable) is the emphasised brand tone — never the muted grayscale look.
    expect(badge).not.toHaveClass('grayscale');
  });

  it('renders a muted above-EU badge when euThreshold=above_eu', () => {
    renderWithI18n(<TenderResultCard tender={fixture({ euThreshold: 'above_eu' })} />);
    const badge = screen.getByTestId('tender-eu-threshold');
    expect(badge).toHaveTextContent(/above/i);
    // "above" recedes via grayscale (no gray token exists — per frontend rules).
    expect(badge).toHaveClass('grayscale');
  });

  it('renders no badge when euThreshold is empty (the guardrail)', () => {
    renderWithI18n(<TenderResultCard tender={fixture({ euThreshold: '' })} />);
    expect(screen.queryByTestId('tender-eu-threshold')).toBeNull();
  });

  it('renders no badge for an unknown band value', () => {
    renderWithI18n(<TenderResultCard tender={fixture({ euThreshold: 'in_band' })} />);
    expect(screen.queryByTestId('tender-eu-threshold')).toBeNull();
  });
});
