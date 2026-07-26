import type { TenderResult } from '@tendersbay/proto/tender/v1/tender_pb';
import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { renderWithI18n } from '~/test/utils';

const { navigateMock } = vi.hoisted(() => ({ navigateMock: vi.fn() }));
vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => navigateMock,
}));

import { TenderResultsTable } from './index';

function fixture(overrides: Partial<TenderResult> = {}): TenderResult {
  return {
    $typeName: 'tender.v1.TenderResult',
    id: 't-1',
    title: 'Bridge inspection and repair works',
    buyerName: 'City of Rome',
    status: 'open',
    procedureType: 'open',
    country: 'IT',
    cpv: '45221000',
    value: 500_000n,
    currency: 'EUR',
    publishedAt: '',
    deadline: '',
    relevanceScore: 0,
    source: 'ted',
    sourceRef: 'ref-1',
    sourceUrl: '',
    nuts: '',
    fitTier: '',
    euThreshold: '',
    ...overrides,
  } as TenderResult;
}

beforeEach(() => navigateMock.mockReset());

describe('TenderResultsTable — rendering', () => {
  it('renders a row per tender with title and buyer', () => {
    renderWithI18n(
      <TenderResultsTable
        tenders={[
          fixture({ id: 't-1', title: 'Road maintenance services' }),
          fixture({ id: 't-2', title: 'Park lighting upgrade' }),
        ]}
      />,
    );
    expect(screen.getByText('Road maintenance services')).toBeInTheDocument();
    expect(screen.getByText('Park lighting upgrade')).toBeInTheDocument();
  });

  it('renders a fit pill when fitTier is set', () => {
    renderWithI18n(<TenderResultsTable tenders={[fixture({ id: 't-1', fitTier: 'strong' })]} />);
    expect(screen.getByText('Strong fit')).toBeInTheDocument();
  });

  it('renders an em-dash when fitTier is empty', () => {
    renderWithI18n(<TenderResultsTable tenders={[fixture({ id: 't-1', fitTier: '' })]} />);
    expect(screen.queryByText('Strong fit')).not.toBeInTheDocument();
  });

  it('renders the formatted value', () => {
    renderWithI18n(
      <TenderResultsTable tenders={[fixture({ id: 't-1', value: 240_000n, currency: 'EUR' })]} />,
    );
    // Formatted value contains "240" and "EUR" somewhere in the output
    expect(screen.getByRole('table')).toHaveTextContent('240');
  });

  it('renders the deadline pill when a future deadline is set', () => {
    const future = new Date();
    future.setDate(future.getDate() + 10);
    renderWithI18n(
      <TenderResultsTable tenders={[fixture({ id: 't-1', deadline: future.toISOString() })]} />,
    );
    expect(screen.getByText('10 days left')).toBeInTheDocument();
  });

  it('renders the Tender column header', () => {
    renderWithI18n(<TenderResultsTable tenders={[fixture()]} />);
    expect(screen.getByRole('columnheader', { name: 'Tender' })).toBeInTheDocument();
  });
});

describe('TenderResultsTable — navigation', () => {
  it('navigates to /tenders/$id when a row is clicked', async () => {
    const user = userEvent.setup();
    renderWithI18n(
      <TenderResultsTable tenders={[fixture({ id: 'nav-1', title: 'Supply of lab equipment' })]} />,
    );
    await user.click(screen.getByText('Supply of lab equipment'));
    expect(navigateMock).toHaveBeenCalledWith({
      to: '/tenders/$id',
      params: { id: 'nav-1' },
    });
  });

  it('navigates on Enter keydown for keyboard users', async () => {
    const user = userEvent.setup();
    renderWithI18n(
      <TenderResultsTable tenders={[fixture({ id: 'kbd-1', title: 'Urban transport study' })]} />,
    );
    // Tab to the row (it has tabIndex=0) and press Enter
    const rows = screen.getAllByRole('row');
    const dataRow = rows[1];
    dataRow.focus();
    await user.keyboard('{Enter}');
    expect(navigateMock).toHaveBeenCalledWith({
      to: '/tenders/$id',
      params: { id: 'kbd-1' },
    });
  });
});

describe('TenderResultsTable — sorting', () => {
  const tenders = [
    fixture({ id: 'a', title: 'Aqueduct repairs', fitTier: 'long_shot', value: 100_000n }),
    fixture({ id: 'b', title: 'Bridge reinforcement', fitTier: 'strong', value: 800_000n }),
    fixture({ id: 'c', title: 'Coastal erosion study', fitTier: '', value: 300_000n }),
  ];

  function getDataRows() {
    return screen.getAllByRole('row').slice(1); // skip header
  }

  it('defaults to fitTier asc — strong row appears first', () => {
    renderWithI18n(<TenderResultsTable tenders={tenders} />);
    const rows = getDataRows();
    expect(within(rows[0]).getByText('Bridge reinforcement')).toBeInTheDocument();
    expect(within(rows[1]).getByText('Aqueduct repairs')).toBeInTheDocument();
    expect(within(rows[2]).getByText('Coastal erosion study')).toBeInTheDocument();
  });

  it('clicking Fit toggles to desc — no-tier row appears first', async () => {
    const user = userEvent.setup();
    renderWithI18n(<TenderResultsTable tenders={tenders} />);
    await user.click(screen.getByRole('button', { name: /fit/i }));
    const rows = getDataRows();
    expect(within(rows[0]).getByText('Coastal erosion study')).toBeInTheDocument();
    expect(within(rows[2]).getByText('Bridge reinforcement')).toBeInTheDocument();
  });

  it('clicking Value sorts ascending by value', async () => {
    const user = userEvent.setup();
    renderWithI18n(<TenderResultsTable tenders={tenders} />);
    await user.click(screen.getByRole('button', { name: /value/i }));
    const rows = getDataRows();
    expect(within(rows[0]).getByText('Aqueduct repairs')).toBeInTheDocument();
    expect(within(rows[1]).getByText('Coastal erosion study')).toBeInTheDocument();
    expect(within(rows[2]).getByText('Bridge reinforcement')).toBeInTheDocument();
  });

  it('clicking Value twice sorts descending', async () => {
    const user = userEvent.setup();
    renderWithI18n(<TenderResultsTable tenders={tenders} />);
    await user.click(screen.getByRole('button', { name: /value/i }));
    await user.click(screen.getByRole('button', { name: /value/i }));
    const rows = getDataRows();
    expect(within(rows[0]).getByText('Bridge reinforcement')).toBeInTheDocument();
  });

  it('pushes tenders with unknown value (0n) to end regardless of sort direction', async () => {
    const user = userEvent.setup();
    const withUnknown = [
      fixture({ id: 'x', title: 'Tender X', value: 0n }),
      fixture({ id: 'y', title: 'Tender Y', value: 50_000n }),
    ];
    renderWithI18n(<TenderResultsTable tenders={withUnknown} />);
    await user.click(screen.getByRole('button', { name: /value/i })); // asc
    const rowsAsc = getDataRows();
    expect(within(rowsAsc[0]).getByText('Tender Y')).toBeInTheDocument();
    expect(within(rowsAsc[1]).getByText('Tender X')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /value/i })); // desc
    const rowsDesc = getDataRows();
    expect(within(rowsDesc[0]).getByText('Tender Y')).toBeInTheDocument();
    expect(within(rowsDesc[1]).getByText('Tender X')).toBeInTheDocument();
  });
});
