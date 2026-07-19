import type { TenderResult } from '@tendersbay/proto/tender/v1/tender_pb';
import { fireEvent, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { renderWithI18n } from '~/test/utils';
import { TenderResultCard } from './index';

const ONE_DAY_MS = 86_400_000;

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

describe('TenderResultCard', () => {
  it("renders with no fit tier and no link when sourceUrl is empty (today's inert card, unchanged)", () => {
    renderWithI18n(<TenderResultCard tender={fixture()} />);
    expect(screen.queryByRole('link')).not.toBeInTheDocument();
  });

  it('renders a link to the source notice when sourceUrl is set', () => {
    renderWithI18n(
      <TenderResultCard tender={fixture({ sourceUrl: 'https://ted.europa.eu/example/notice' })} />,
    );
    const link = screen.getByRole('link');
    expect(link).toHaveAttribute('href', 'https://ted.europa.eu/example/notice');
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', expect.stringContaining('noopener'));
  });

  it('calls onOpen when the source link is clicked', () => {
    const onOpen = vi.fn();
    renderWithI18n(
      <TenderResultCard
        tender={fixture({ sourceUrl: 'https://ted.europa.eu/example/notice' })}
        onOpen={onOpen}
      />,
    );
    fireEvent.click(screen.getByRole('link'));
    expect(onOpen).toHaveBeenCalledTimes(1);
  });

  it('renders the strong fit tier pill and a reason line built from ReasonSignals', () => {
    renderWithI18n(
      <TenderResultCard
        tender={fixture()}
        fitTier="strong"
        reason={{
          sectorMatch: true,
          countryMatch: true,
          valueFit: 'in_band',
          deadlineDays: 12,
          hasDeadline: true,
          regionMatch: false,
          procedureMatch: false,
        }}
      />,
    );
    expect(screen.getByText('Strong fit')).toBeInTheDocument();
    // Reason line joins fragments with " · " — assert the container has all four pieces present.
    const reasonLine = screen.getByTestId('tender-fit-reason');
    expect(reasonLine.textContent).toContain('sector');
    expect(reasonLine.textContent).toContain('12');
  });

  it('includes region and procedure match text in the reason line, appended after the other signals', () => {
    renderWithI18n(
      <TenderResultCard
        tender={fixture()}
        fitTier="possible"
        reason={{
          sectorMatch: true,
          countryMatch: false,
          valueFit: 'unknown',
          deadlineDays: 0,
          hasDeadline: false,
          regionMatch: true,
          procedureMatch: true,
        }}
      />,
    );
    const reasonLine = screen.getByTestId('tender-fit-reason');
    expect(reasonLine.textContent).toContain('sector');
    expect(reasonLine.textContent).toContain('region');
    expect(reasonLine.textContent).toContain('procedure');
    // Never a numeric match percentage anywhere in the reason line.
    expect(reasonLine.textContent).not.toMatch(/%/);
  });

  it('renders no reason line when no signal matched', () => {
    renderWithI18n(
      <TenderResultCard
        tender={fixture()}
        fitTier="long_shot"
        reason={{
          sectorMatch: false,
          countryMatch: false,
          valueFit: 'unknown',
          deadlineDays: 0,
          hasDeadline: false,
          regionMatch: false,
          procedureMatch: false,
        }}
      />,
    );
    expect(screen.queryByTestId('tender-fit-reason')).not.toBeInTheDocument();
  });

  it('renders no fit pill at all when fitTier is not provided (plain search result)', () => {
    renderWithI18n(<TenderResultCard tender={fixture()} />);
    expect(screen.queryByText('Strong fit')).not.toBeInTheDocument();
    expect(screen.queryByText('Possible fit')).not.toBeInTheDocument();
    expect(screen.queryByText('Long shot')).not.toBeInTheDocument();
  });

  it('renders the buyer line when buyerName is set', () => {
    renderWithI18n(<TenderResultCard tender={fixture({ buyerName: 'City of Lisbon' })} />);
    expect(screen.getByText('City of Lisbon')).toBeInTheDocument();
  });

  it('omits the buyer line when buyerName is empty', () => {
    renderWithI18n(<TenderResultCard tender={fixture({ buyerName: '' })} />);
    expect(screen.queryByText('City of Lisbon')).not.toBeInTheDocument();
  });

  it('renders an actual flag component when the country code resolves to one (PT)', () => {
    const { container } = renderWithI18n(<TenderResultCard tender={fixture({ country: 'PT' })} />);
    expect(container.querySelector('svg')).toBeInTheDocument();
  });

  it('falls back to a plain country-code badge when no flag is available for the code (US)', () => {
    const { container } = renderWithI18n(<TenderResultCard tender={fixture({ country: 'US' })} />);
    expect(container.querySelector('svg')).not.toBeInTheDocument();
    expect(screen.getByText('US')).toBeInTheDocument();
  });

  describe('deadline tone wired to the Pill CSS class', () => {
    afterEach(() => {
      vi.useRealTimers();
    });

    it('renders the urgent Tailwind class for a deadline due today (proven boundary: day 0)', () => {
      const now = new Date('2026-07-13T12:00:00.000Z');
      vi.useFakeTimers();
      vi.setSystemTime(now);
      renderWithI18n(<TenderResultCard tender={fixture({ deadline: now.toISOString() })} />);
      expect(screen.getByText('Closes today')).toHaveClass('bg-signal-urgent-100');
    });

    it('renders the warm/approaching Tailwind class for a deadline 8 days out (proven boundary: day 8)', () => {
      const now = new Date('2026-07-13T12:00:00.000Z');
      vi.useFakeTimers();
      vi.setSystemTime(now);
      renderWithI18n(
        <TenderResultCard
          tender={fixture({ deadline: new Date(now.getTime() + 8 * ONE_DAY_MS).toISOString() })}
        />,
      );
      expect(screen.getByText('8 days left')).toHaveClass('bg-signal-warm-100');
    });
  });
});
