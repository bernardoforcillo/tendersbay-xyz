import type { CpvMatch, TenderFilters, TenderResult } from '@tendersbay/proto/tender/v1/tender_pb';
import { fireEvent, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { NuqsTestingAdapter } from 'nuqs/adapters/testing';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { SearchMeta } from '~/features/account/components/organisms/tender-feed';
import { useChatStore } from '~/store/chat';
import { renderWithI18n } from '~/test/utils';

const { searchMock, loadMoreMock, useTenderSearchMock } = vi.hoisted(() => ({
  searchMock: vi.fn(),
  loadMoreMock: vi.fn(),
  useTenderSearchMock: vi.fn(),
}));

/** The search metadata shape the hook always returns — see `SearchMeta`. */
const EMPTY_META: SearchMeta = {
  mode: 'hybrid',
  degraded: false,
  appliedQuery: '',
  appliedCpv: [],
  countryFacets: [],
  statusFacets: [],
  cpvDivisionFacets: [],
};

function cpvMatch(overrides: Partial<CpvMatch> = {}): CpvMatch {
  return {
    $typeName: 'tender.v1.CpvMatch',
    code: '90919200',
    label: 'Servizi di pulizia di uffici',
    language: 'it',
    score: 0.9,
    ...overrides,
  } as CpvMatch;
}

function appliedTenderFilters(overrides: Partial<TenderFilters> = {}): TenderFilters {
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

const { navigateMock } = vi.hoisted(() => ({ navigateMock: vi.fn() }));
vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => navigateMock,
  Link: ({ children, href }: { children: ReactNode; href?: string }) => (
    <a href={href}>{children}</a>
  ),
}));

const { useClientShortlistMock } = vi.hoisted(() => ({ useClientShortlistMock: vi.fn() }));
vi.mock('~/features/account/components/pages/explore/use-client-shortlist', () => ({
  useClientShortlist: () => useClientShortlistMock(),
}));

vi.mock('~/store/workspace', () => ({
  useWorkspaceStore: (selector: (s: { currentWorkspaceId: string | null }) => unknown) =>
    selector({ currentWorkspaceId: 'ws-1' }),
}));

const captureMock = vi.fn();
vi.mock('posthog-js/react', () => ({ usePostHog: () => ({ capture: captureMock }) }));

vi.mock('~/features/account/components/organisms/tender-feed', async (importOriginal) => {
  const actual =
    await importOriginal<typeof import('~/features/account/components/organisms/tender-feed')>();
  return {
    ...actual,
    useTenderSearch: () => useTenderSearchMock(),
  };
});

vi.mock('~/features/account/components/templates/account-layout', () => ({
  AccountLayout: ({ children }: { children: ReactNode }) => <div>{children}</div>,
}));

vi.mock('~/features/account/components/organisms', async (importOriginal) => {
  const actual = await importOriginal<typeof import('~/features/account/components/organisms')>();
  return {
    ...actual,
    PageHeader: () => <div data-testid="page-header" />,
  };
});

vi.mock('~/features/tenders', () => ({
  useTenderLink:
    () => (id: string, children: ReactNode, _className?: string, onClick?: () => void) => (
      <a href={`/tenders/${id}`} onClick={onClick}>
        {children}
      </a>
    ),
}));

import { AccountTendersPage } from './index';

function renderTenders(searchParams = '') {
  return renderWithI18n(
    <NuqsTestingAdapter searchParams={searchParams}>
      <AccountTendersPage />
    </NuqsTestingAdapter>,
  );
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

function recommendedFixture(overrides: Partial<TenderResult> = {}) {
  return {
    tender: fixture(overrides),
    fitTier: 'strong',
    reason: {
      sectorMatch: true,
      countryMatch: true,
      valueFit: 'in_band',
      deadlineDays: 10,
      hasDeadline: true,
    },
  };
}

type HookReturn = {
  results: TenderResult[];
  hasMore: boolean;
  loading: boolean;
  error: string | null;
  meta: typeof EMPTY_META;
  search: (query: string) => Promise<void>;
  loadMore: () => Promise<void>;
};

function mockHook(overrides: Partial<HookReturn> = {}) {
  useTenderSearchMock.mockReturnValue({
    results: [],
    hasMore: false,
    loading: false,
    error: null,
    meta: EMPTY_META,
    search: searchMock,
    loadMore: loadMoreMock,
    ...overrides,
  });
}

async function submit(user: ReturnType<typeof userEvent.setup>, query: string) {
  const input = screen.getByRole('textbox', { name: 'Search' });
  await user.type(input, `${query}{Enter}`);
}

describe('AccountTendersPage — search', () => {
  beforeEach(() => {
    searchMock.mockReset();
    loadMoreMock.mockReset();
    navigateMock.mockReset();
    useChatStore.setState({ messages: [], currentChatId: null, draft: null });
    useClientShortlistMock.mockReturnValue({
      results: [],
      needsProfile: false,
      loading: false,
      error: null,
      refetch: vi.fn(),
    });
    mockHook();
  });

  it('shows the greeting hero and no results section before any search has run', () => {
    renderTenders();
    expect(screen.getByText('What are you bidding on today?')).toBeInTheDocument();
    expect(screen.queryByText('No tenders found')).not.toBeInTheDocument();
    expect(screen.queryByText('Load more')).not.toBeInTheDocument();
  });

  it('submitting a query calls search with the trimmed value', async () => {
    const user = userEvent.setup();
    renderTenders();
    await submit(user, '  roads  ');
    expect(searchMock).toHaveBeenCalledWith('roads', {}, 'ws-1', 'relevance');
  });

  it('is a no-op on an empty (whitespace-only) submit', async () => {
    const user = userEvent.setup();
    renderTenders();
    await submit(user, '   ');
    expect(searchMock).not.toHaveBeenCalled();
    expect(screen.queryByText('No tenders found')).not.toBeInTheDocument();
  });

  it('renders the count line, result cards, and an enabled Load more when hasMore', async () => {
    mockHook({
      results: [fixture(), fixture({ id: 't-2', title: 'Bridge inspection services' })],
      hasMore: true,
    });
    const user = userEvent.setup();
    renderTenders();
    await submit(user, 'roads');

    expect(screen.getByText('2 tenders')).toBeInTheDocument();
    expect(screen.getByText('Supply of road maintenance services')).toBeInTheDocument();
    expect(screen.getByText('Bridge inspection services')).toBeInTheDocument();

    const loadMoreBtn = screen.getByRole('button', { name: 'Load more' });
    expect(loadMoreBtn).toBeEnabled();
    await user.click(loadMoreBtn);
    expect(loadMoreMock).toHaveBeenCalledTimes(1);
  });

  it('runs a manual search scoped to the current workspace', async () => {
    const user = userEvent.setup();
    renderTenders();
    await submit(user, 'roads');
    expect(searchMock).toHaveBeenCalledWith('roads', {}, 'ws-1', 'relevance');
  });

  it('renders the fit-tier pill on a manual search result when the backend annotated it', async () => {
    mockHook({
      results: [
        fixture({
          id: 't-fit',
          fitTier: 'strong',
          reason: {
            $typeName: 'tender.v1.ReasonSignals',
            sectorMatch: true,
            countryMatch: true,
            valueFit: 'in_band',
            deadlineDays: 10,
            hasDeadline: true,
            regionMatch: false,
            procedureMatch: false,
          },
        }),
      ],
      hasMore: false,
    });
    const user = userEvent.setup();
    renderTenders();
    await submit(user, 'roads');

    expect(screen.getByText('Strong fit')).toBeInTheDocument();
  });

  it('renders a manual search result with no pill when fitTier is empty', async () => {
    mockHook({ results: [fixture({ id: 't-no-fit' })], hasMore: false });
    const user = userEvent.setup();
    renderTenders();
    await submit(user, 'roads');

    expect(screen.queryByText('Strong fit')).not.toBeInTheDocument();
    expect(screen.queryByText('Possible fit')).not.toBeInTheDocument();
    expect(screen.queryByText('Long shot')).not.toBeInTheDocument();
  });

  it('disables Load more while a request is in flight', async () => {
    mockHook({ results: [fixture()], hasMore: true, loading: true });
    const user = userEvent.setup();
    renderTenders();
    await submit(user, 'roads');

    const loadMoreBtn = screen.getByRole('button', { name: 'Load more' });
    expect(loadMoreBtn).toBeDisabled();
    await user.click(loadMoreBtn);
    expect(loadMoreMock).not.toHaveBeenCalled();
  });

  it('shows the empty state when a search returns zero results', async () => {
    mockHook({ results: [], hasMore: false, loading: false });
    const user = userEvent.setup();
    renderTenders();
    await submit(user, 'zzz');

    expect(screen.getByText('No tenders found')).toBeInTheDocument();
    expect(screen.getByText('Try a broader query or fewer filters.')).toBeInTheDocument();
  });

  it('shows an error banner when the search fails', async () => {
    mockHook({ results: [], error: 'boom' });
    const user = userEvent.setup();
    renderTenders();
    await submit(user, 'zzz');

    expect(screen.getByRole('alert')).toHaveTextContent(
      'Search is unavailable right now — try again in a moment.',
    );
  });

  it('seeds a search from ?q= on mount', () => {
    useTenderSearchMock.mockReturnValue({
      results: [],
      hasMore: false,
      loading: false,
      error: null,
      meta: EMPTY_META,
      search: searchMock,
      loadMore: loadMoreMock,
    });
    renderTenders('?q=roads');
    expect(searchMock).toHaveBeenCalledWith('roads', expect.anything(), 'ws-1', 'relevance');
  });

  it('navigates to /explore when a chat draft arrives', () => {
    useChatStore.setState({ draft: 'find me tenders in IT' });
    renderTenders();
    expect(navigateMock).toHaveBeenCalledWith({ to: '/explore' });
  });

  it('runs a filters-only search when a filter is set with an empty query', async () => {
    const user = userEvent.setup();
    renderTenders();
    await user.click(screen.getByRole('button', { name: 'Italy' }));
    expect(searchMock).toHaveBeenCalledWith('', { countries: ['IT'] }, 'ws-1', 'relevance');
  });

  it('maps the sector selection to a CPV prefix', async () => {
    const user = userEvent.setup();
    renderTenders();
    await user.click(screen.getByRole('button', { name: 'Construction' }));
    expect(searchMock).toHaveBeenCalledWith('', { cpvPrefixes: ['45'] }, 'ws-1', 'relevance');
  });

  it('includes the active filters when searching with a query', async () => {
    const user = userEvent.setup();
    renderTenders('?q=roads');
    await user.click(screen.getByRole('button', { name: 'Open' }));
    expect(searchMock).toHaveBeenLastCalledWith(
      'roads',
      { statuses: ['open'] },
      'ws-1',
      'relevance',
    );
  });

  // Multi-select is the point of the chips: a bidder working in two countries
  // shouldn't have to run the same search twice.
  it('accumulates multiple values on one facet', async () => {
    const user = userEvent.setup();
    renderTenders();
    await user.click(screen.getByRole('button', { name: 'Italy' }));
    await user.click(screen.getByRole('button', { name: 'Germany' }));
    expect(searchMock).toHaveBeenLastCalledWith(
      '',
      { countries: ['IT', 'DE'] },
      'ws-1',
      'relevance',
    );
  });

  it('deselects a value when its chip is clicked again', async () => {
    const user = userEvent.setup();
    renderTenders('?q=roads');
    await user.click(screen.getByRole('button', { name: 'Italy' }));
    await user.click(screen.getByRole('button', { name: 'Italy' }));
    expect(searchMock).toHaveBeenLastCalledWith('roads', {}, 'ws-1', 'relevance');
  });

  it('threads the chosen sort into the search', async () => {
    const user = userEvent.setup();
    renderTenders('?q=roads');
    await user.selectOptions(screen.getByLabelText('Sort by'), 'deadline');
    expect(searchMock).toHaveBeenLastCalledWith('roads', {}, 'ws-1', 'deadline');
  });

  // A half-typed number must not become a `0` bound that silently empties the
  // results.
  it('ignores a value bound until it parses as a number', async () => {
    const user = userEvent.setup();
    renderTenders('?q=roads');
    await user.type(screen.getByLabelText('Max value'), 'abc');
    expect(searchMock).toHaveBeenLastCalledWith('roads', {}, 'ws-1', 'relevance');

    // Typed in whole euros, sent in minor units: the bound is compared against
    // a minor-unit column, so an unscaled 100000 would cap the search at €1,000.
    await user.clear(screen.getByLabelText('Max value'));
    await user.type(screen.getByLabelText('Max value'), '100000');
    expect(searchMock).toHaveBeenLastCalledWith(
      'roads',
      { valueMax: 100_000_00n },
      'ws-1',
      'relevance',
    );
  });

  it('maps a deadline preset to an RFC3339 from/to window', async () => {
    const user = userEvent.setup();
    renderTenders();
    await user.selectOptions(screen.getByLabelText('Deadline'), '7');
    const filters = (searchMock.mock.calls.at(-1)?.[1] ?? {}) as {
      deadlineFrom?: string;
      deadlineTo?: string;
    };
    const from = filters.deadlineFrom ?? '';
    const to = filters.deadlineTo ?? '';
    expect(from).not.toBe('');
    expect(to).not.toBe('');
    expect(new Date(to).getTime()).toBeGreaterThan(new Date(from).getTime());
  });

  it('clears filters and re-runs the search with the query only', async () => {
    const user = userEvent.setup();
    renderTenders('?q=roads');
    await user.click(screen.getByRole('button', { name: 'Italy' }));
    searchMock.mockClear();
    await user.click(screen.getByRole('button', { name: 'Clear all' }));
    expect(searchMock).toHaveBeenCalledWith('roads', {}, 'ws-1', 'relevance');
    expect(screen.getByRole('button', { name: 'Italy' })).toHaveAttribute('aria-pressed', 'false');
  });

  // A search running on one retriever answers the question asked, just less
  // well — saying so beats presenting partial results as complete ones.
  it('warns when the search ran degraded', async () => {
    mockHook({ meta: { ...EMPTY_META, mode: 'lexical', degraded: true } });
    renderTenders('?q=roads');
    expect(
      screen.getByText(/Part of the search is unavailable/, { exact: false }),
    ).toBeInTheDocument();
  });

  it('does not warn when both retrievers answered', async () => {
    mockHook({ meta: { ...EMPTY_META, mode: 'hybrid', degraded: false } });
    renderTenders('?q=roads');
    expect(screen.queryByText(/Part of the search is unavailable/, { exact: false })).toBeNull();
  });

  // AppliedCpvChips and AppliedFilterChips are each unit-tested in isolation,
  // and neither renders its own "Read from your search:" heading any more —
  // the page renders it once, gated on either family having content. Neither
  // component's own test suite can see this: a plain free-text query with no
  // numeric/date/country phrasing (the running example throughout this
  // effort, "pulizie uffici") resolves a CPV match with ZERO applied
  // filters, which is exactly the case a naive "let AppliedFilterChips own
  // the heading" fix would silently drop the heading for.
  describe('the shared "applied" heading', () => {
    it('shows it exactly once for a CPV match with no applied filters', async () => {
      mockHook({
        meta: {
          ...EMPTY_META,
          appliedCpv: [cpvMatch()],
          appliedFilters: appliedTenderFilters(), // every facet empty
        },
      });
      const user = userEvent.setup();
      renderTenders();
      await submit(user, 'pulizie uffici');

      expect(screen.getAllByText('Read from your search:')).toHaveLength(1);
    });

    it('shows it exactly once when both families have content', async () => {
      mockHook({
        meta: {
          ...EMPTY_META,
          appliedCpv: [cpvMatch()],
          appliedFilters: appliedTenderFilters({ valueMax: 100_000n }),
        },
      });
      const user = userEvent.setup();
      renderTenders();
      await submit(user, 'pulizie uffici sotto 100k');

      expect(screen.getAllByText('Read from your search:')).toHaveLength(1);
    });

    it('shows it not at all when neither family has content', async () => {
      mockHook({ meta: EMPTY_META });
      const user = userEvent.setup();
      renderTenders();
      await submit(user, 'roads');

      expect(screen.queryByText('Read from your search:')).toBeNull();
    });
  });
});

describe('AccountTendersPage — client shortlist', () => {
  beforeEach(() => {
    searchMock.mockReset();
    loadMoreMock.mockReset();
    navigateMock.mockReset();
    useChatStore.setState({ messages: [], currentChatId: null, draft: null });
    useClientShortlistMock.mockReturnValue({
      results: [],
      needsProfile: false,
      loading: false,
      error: null,
      refetch: vi.fn(),
    });
    mockHook({ results: [], hasMore: false, loading: false, error: null });
  });

  it('shows the profile setup form directly when needsProfile is true', () => {
    useClientShortlistMock.mockReturnValue({
      results: [],
      needsProfile: true,
      loading: false,
      error: null,
      refetch: vi.fn(),
    });
    renderTenders();
    expect(screen.getByRole('button', { name: /save profile/i })).toBeInTheDocument();
  });

  it('renders the shortlist cards when a profile exists and results are returned', () => {
    useClientShortlistMock.mockReturnValue({
      results: [recommendedFixture({ id: 'r-1' })],
      needsProfile: false,
      loading: false,
      error: null,
      refetch: vi.fn(),
    });
    renderTenders();
    expect(screen.getByText('Strong fit')).toBeInTheDocument();
  });

  it('renders nothing from the shortlist block once a manual search has run', async () => {
    const user = userEvent.setup();
    useClientShortlistMock.mockReturnValue({
      results: [recommendedFixture({ id: 'r-1' })],
      needsProfile: false,
      loading: false,
      error: null,
      refetch: vi.fn(),
    });
    mockHook({ results: [fixture({ id: 's-1' })], hasMore: false, loading: false, error: null });
    renderTenders();

    await submit(user, 'roads');

    expect(screen.queryByText('Strong fit')).not.toBeInTheDocument();
  });

  it('captures client_shortlist_viewed once, with shortlist_size and has_strong', () => {
    captureMock.mockClear();
    useClientShortlistMock.mockReturnValue({
      results: [recommendedFixture({ id: 'r-1' })],
      needsProfile: false,
      loading: false,
      error: null,
      refetch: vi.fn(),
    });
    renderTenders();

    expect(captureMock).toHaveBeenCalledWith('client_shortlist_viewed', {
      location: 'explore_shortlist',
      shortlist_size: 1,
      has_strong: true,
    });
  });

  it('captures shortlist_match_opened with the fit tier when a card is opened', () => {
    captureMock.mockClear();
    useClientShortlistMock.mockReturnValue({
      results: [
        recommendedFixture({ id: 'r-1', sourceUrl: 'https://ted.europa.eu/example/notice' }),
      ],
      needsProfile: false,
      loading: false,
      error: null,
      refetch: vi.fn(),
    });
    renderTenders();

    fireEvent.click(screen.getByRole('link'));

    expect(captureMock).toHaveBeenCalledWith('shortlist_match_opened', {
      location: 'explore_shortlist',
      fit_tier: 'strong',
    });
  });
});
