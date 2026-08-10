import type { TenderFilters } from '@tendersbay/proto/tender/v1/tender_pb';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { renderWithI18n } from '~/test/utils';
import { AppliedFilterChips } from './index';

function applied(overrides: Partial<TenderFilters> = {}): TenderFilters {
  return {
    $typeName: 'tender.v1.TenderFilters',
    country: '',
    cpv: '',
    status: '',
    deadlineFrom: '',
    deadlineTo: '',
    countries: [],
    cpvPrefixes: [],
    statuses: [],
    nutsPrefixes: [],
    buyer: '',
    ...overrides,
  } as TenderFilters;
}

const NOTHING_EXPLICIT = {
  countries: [],
  cpvPrefixes: [],
  statuses: [],
  hasValueBounds: false,
  hasDeadline: false,
};

describe('AppliedFilterChips', () => {
  it('shows a constraint the server read out of the query', () => {
    renderWithI18n(
      <AppliedFilterChips
        applied={applied({ valueMax: 100_000_00n })}
        explicit={NOTHING_EXPLICIT}
        locale="en-IE"
        onClear={vi.fn()}
      />,
    );
    expect(screen.getByText(/up to/)).toBeInTheDocument();
  });

  // These chips exist to make INFERRED constraints visible. Echoing back a
  // filter the user picked themselves would suggest the search added something
  // it didn't.
  it('does not repeat a filter the user set explicitly', () => {
    renderWithI18n(
      <AppliedFilterChips
        applied={applied({ countries: ['IT'], statuses: ['open'] })}
        explicit={{ ...NOTHING_EXPLICIT, countries: ['IT'], statuses: ['open'] }}
        locale="en-IE"
        onClear={vi.fn()}
      />,
    );
    expect(screen.queryByText('Italy')).toBeNull();
    expect(screen.queryByText('Open')).toBeNull();
  });

  it('shows only the facets the user did not set', () => {
    renderWithI18n(
      <AppliedFilterChips
        applied={applied({ countries: ['IT'], statuses: ['open'] })}
        explicit={{ ...NOTHING_EXPLICIT, countries: ['IT'] }}
        locale="en-IE"
        onClear={vi.fn()}
      />,
    );
    expect(screen.queryByText('Italy')).toBeNull();
    expect(screen.getByText('Open')).toBeInTheDocument();
  });

  it('renders nothing when the search inferred no constraints', () => {
    const { container } = renderWithI18n(
      <AppliedFilterChips
        applied={applied()}
        explicit={NOTHING_EXPLICIT}
        locale="en-IE"
        onClear={vi.fn()}
      />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('renders nothing before the first search', () => {
    const { container } = renderWithI18n(
      <AppliedFilterChips explicit={NOTHING_EXPLICIT} locale="en-IE" onClear={vi.fn()} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  // A filter the user can't see is one they can't correct, so the undo has to
  // be right there next to what it undoes.
  it('offers an undo that clears the query the constraints came from', async () => {
    const user = userEvent.setup();
    const onClear = vi.fn();
    renderWithI18n(
      <AppliedFilterChips
        applied={applied({ valueMax: 100_000_00n })}
        explicit={NOTHING_EXPLICIT}
        locale="en-IE"
        onClear={onClear}
      />,
    );
    await user.click(screen.getByRole('button', { name: 'Clear' }));
    expect(onClear).toHaveBeenCalledOnce();
  });

  it('shows a range when both bounds were inferred', () => {
    renderWithI18n(
      <AppliedFilterChips
        applied={applied({ valueMin: 50_000_00n, valueMax: 200_000_00n })}
        explicit={NOTHING_EXPLICIT}
        locale="en-IE"
        onClear={vi.fn()}
      />,
    );
    // Bounds arrive in minor units and must read back as the whole euros the
    // user typed — a chip saying "5,000,000–20,000,000" would look like the
    // search had misread the query it is meant to be echoing.
    expect(screen.getByText('50,000–200,000')).toBeInTheDocument();
  });

  it('ignores an unparseable deadline rather than rendering an invalid date', () => {
    renderWithI18n(
      <AppliedFilterChips
        applied={applied({ deadlineTo: 'not-a-date' })}
        explicit={NOTHING_EXPLICIT}
        locale="en-IE"
        onClear={vi.fn()}
      />,
    );
    expect(screen.queryByText(/Invalid/)).toBeNull();
  });
});
