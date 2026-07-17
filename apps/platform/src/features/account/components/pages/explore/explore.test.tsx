import type { TenderResult } from '@tendersbay/proto/tender/v1/tender_pb';
import { screen } from '@testing-library/react';
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
    expect(searchMock).toHaveBeenCalledWith('roads');
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
    expect(searchMock).toHaveBeenCalledWith('roads');
  });
});
