import type { TenderResult } from '@tendersbay/proto/tender/v1/tender_pb';
import { fireEvent, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { NuqsTestingAdapter } from 'nuqs/adapters/testing';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useChatStore } from '~/store/chat';
import { renderWithI18n } from '~/test/utils';

const { searchMock, loadMoreMock, useTenderSearchMock } = vi.hoisted(() => ({
  searchMock: vi.fn(),
  loadMoreMock: vi.fn(),
  useTenderSearchMock: vi.fn(),
}));

const { useClientShortlistMock } = vi.hoisted(() => ({ useClientShortlistMock: vi.fn() }));
vi.mock('./use-client-shortlist', () => ({ useClientShortlist: () => useClientShortlistMock() }));

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
    ChatWindow: () => <div data-testid="chat-window" />,
    PageHeader: () => <div data-testid="page-header" />,
  };
});

vi.mock('~/features/tenders', () => ({
  useTenderLink: () => (_id: string, children: ReactNode) => children,
}));

import { AccountExplorePage } from './index';

function renderExplore(searchParams = '') {
  return renderWithI18n(
    <NuqsTestingAdapter searchParams={searchParams}>
      <AccountExplorePage />
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
  search: (query: string) => Promise<void>;
  loadMore: () => Promise<void>;
};

function mockHook(overrides: Partial<HookReturn> = {}) {
  useTenderSearchMock.mockReturnValue({
    results: [],
    hasMore: false,
    loading: false,
    error: null,
    search: searchMock,
    loadMore: loadMoreMock,
    ...overrides,
  });
}

async function submit(user: ReturnType<typeof userEvent.setup>, query: string) {
  const input = screen.getByRole('textbox', { name: 'Search' });
  await user.type(input, `${query}{Enter}`);
}

describe('AccountExplorePage — search mode', () => {
  beforeEach(() => {
    searchMock.mockReset();
    loadMoreMock.mockReset();
    useChatStore.setState({ messages: [], currentChatId: null, draft: null });
    // The shortlist hook is mocked at module scope (also used by the
    // "client shortlist" describe below) — give it a neutral default here
    // so these pre-existing search-mode tests, which don't care about the
    // shortlist at all, don't crash on an undefined mock return.
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
    renderExplore();
    expect(screen.getByText('What are you bidding on today?')).toBeInTheDocument();
    expect(screen.queryByText('No tenders found')).not.toBeInTheDocument();
    expect(screen.queryByText('Load more')).not.toBeInTheDocument();
  });

  it('submitting a query calls search with the trimmed value', async () => {
    const user = userEvent.setup();
    renderExplore();
    await submit(user, '  roads  ');
    expect(searchMock).toHaveBeenCalledWith('roads', {}, 'ws-1');
  });

  it('is a no-op on an empty (whitespace-only) submit', async () => {
    const user = userEvent.setup();
    renderExplore();
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
    renderExplore();
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
    renderExplore();
    await submit(user, 'roads');
    // The `~/store/workspace` mock at module scope fixes currentWorkspaceId to 'ws-1'.
    expect(searchMock).toHaveBeenCalledWith('roads', {}, 'ws-1');
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
    renderExplore();
    await submit(user, 'roads');

    expect(screen.getByText('Strong fit')).toBeInTheDocument();
  });

  it('renders a manual search result with no pill when fitTier is empty (annotation did not apply)', async () => {
    // Never fabricate a signal: an unset/empty fitTier must render the card exactly as before.
    mockHook({ results: [fixture({ id: 't-no-fit' })], hasMore: false });
    const user = userEvent.setup();
    renderExplore();
    await submit(user, 'roads');

    expect(screen.queryByText('Strong fit')).not.toBeInTheDocument();
    expect(screen.queryByText('Possible fit')).not.toBeInTheDocument();
    expect(screen.queryByText('Long shot')).not.toBeInTheDocument();
  });

  it('disables Load more while a request is in flight', async () => {
    mockHook({ results: [fixture()], hasMore: true, loading: true });
    const user = userEvent.setup();
    renderExplore();
    await submit(user, 'roads');

    const loadMoreBtn = screen.getByRole('button', { name: 'Load more' });
    expect(loadMoreBtn).toBeDisabled();
    await user.click(loadMoreBtn);
    expect(loadMoreMock).not.toHaveBeenCalled();
  });

  it('shows the empty state when a search returns zero results', async () => {
    mockHook({ results: [], hasMore: false, loading: false });
    const user = userEvent.setup();
    renderExplore();
    await submit(user, 'zzz');

    expect(screen.getByText('No tenders found')).toBeInTheDocument();
    expect(screen.getByText('Try a broader query or fewer filters.')).toBeInTheDocument();
  });

  it('shows an error banner when the search fails', async () => {
    mockHook({ results: [], error: 'boom' });
    const user = userEvent.setup();
    renderExplore();
    await submit(user, 'zzz');

    expect(screen.getByRole('alert')).toHaveTextContent(
      'Search is unavailable right now — try again in a moment.',
    );
  });

  it('leaves chat mode untouched when there is prior chat history', () => {
    useChatStore.setState({
      messages: [{ id: 'm1', role: 'user', content: 'hi', createdAt: '' }],
    });
    renderExplore();
    expect(screen.getByTestId('chat-window')).toBeInTheDocument();
    expect(screen.queryByRole('textbox', { name: 'Search' })).not.toBeInTheDocument();
  });

  it('seeds a search from ?q= on mount', () => {
    useTenderSearchMock.mockReturnValue({
      results: [],
      hasMore: false,
      loading: false,
      error: null,
      search: searchMock,
      loadMore: loadMoreMock,
    });
    renderExplore('?q=roads');
    expect(searchMock).toHaveBeenCalledWith('roads', expect.anything(), 'ws-1');
  });

  it('runs a filters-only search when a filter is set with an empty query', async () => {
    const user = userEvent.setup();
    renderExplore();
    await user.selectOptions(screen.getByLabelText('Country'), 'IT');
    expect(searchMock).toHaveBeenCalledWith('', { country: 'IT' }, 'ws-1');
  });

  it('maps the sector selection to a CPV prefix', async () => {
    const user = userEvent.setup();
    renderExplore();
    await user.selectOptions(screen.getByLabelText('Sector'), 'construction');
    expect(searchMock).toHaveBeenCalledWith('', { cpv: '45' }, 'ws-1');
  });

  it('includes the active filters when searching with a query', async () => {
    const user = userEvent.setup();
    // Seed the query via ?q= rather than typing: nuqs commits the text-box value
    // asynchronously, so typing-then-acting races the URL state under the test's
    // fast synthetic events. A URL-seeded query is settled from mount.
    renderExplore('?q=roads');
    await user.selectOptions(screen.getByLabelText('Status'), 'open');
    expect(searchMock).toHaveBeenLastCalledWith('roads', { status: 'open' }, 'ws-1');
  });

  it('maps a deadline preset to an RFC3339 from/to window', async () => {
    const user = userEvent.setup();
    renderExplore();
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
    // Seed the query via ?q= (see note above — nuqs commits typing asynchronously).
    renderExplore('?q=roads');
    await user.selectOptions(screen.getByLabelText('Country'), 'IT');
    searchMock.mockClear();
    await user.click(screen.getByRole('button', { name: 'Clear all' }));
    expect(searchMock).toHaveBeenCalledWith('roads', {}, 'ws-1');
    expect(screen.getByLabelText('Country')).toHaveValue('');
  });
});

describe('client shortlist (Explore default state)', () => {
  beforeEach(() => {
    searchMock.mockReset();
    loadMoreMock.mockReset();
    useChatStore.setState({ messages: [], currentChatId: null, draft: null });
    useClientShortlistMock.mockReturnValue({
      results: [],
      needsProfile: false,
      loading: false,
      error: null,
      refetch: vi.fn(),
    });
    mockHook({ results: [], hasMore: false, loading: false, error: null }); // no manual search has run
  });

  it('shows the profile setup form directly when needsProfile is true', () => {
    // v1.0 skips a separate "set up profile" CTA button — needsProfile:true
    // renders ClientProfileForm immediately (the fewest clicks to a first
    // match); there is no "edit an existing profile" affordance yet (see
    // Step 7's note — deliberately deferred, not a gap in this task).
    useClientShortlistMock.mockReturnValue({
      results: [],
      needsProfile: true,
      loading: false,
      error: null,
      refetch: vi.fn(),
    });
    renderExplore();
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
    renderExplore();
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
    renderExplore();

    await submit(user, 'roads');

    // Once `searched` is true, the manual-results branch owns that slot —
    // the shortlist's own "Strong fit" pill must not double-render there.
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
    renderExplore();

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
    renderExplore();

    fireEvent.click(screen.getByRole('link'));

    expect(captureMock).toHaveBeenCalledWith('shortlist_match_opened', {
      location: 'explore_shortlist',
      fit_tier: 'strong',
    });
  });
});
