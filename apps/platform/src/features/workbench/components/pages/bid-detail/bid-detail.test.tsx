import { act, render, screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, opt?: unknown) =>
      typeof opt === 'string' ? opt : ((opt as { defaultValue?: string })?.defaultValue ?? key),
    i18n: { language: 'en-ie' },
  }),
}));
const { flags } = vi.hoisted(() => ({ flags: { dgue: false as boolean | undefined } }));
vi.mock('posthog-js/react', () => ({
  usePostHog: () => ({ capture: vi.fn() }),
  useFeatureFlagEnabled: () => flags.dgue,
}));
vi.mock('@tanstack/react-router', () => ({
  useParams: () => ({ bidId: 'b1', workspaceId: 'ws1', workbenchId: 'wb1' }),
  Link: ({ children }: { children: ReactNode }) => <a href="/">{children}</a>,
}));
vi.mock('~/features/workbench/context', () => ({
  useWorkbenchContext: () => ({
    workbenchId: 'wb1',
    workbench: { workspaceId: 'ws1' },
    myPermissions: 2n, // ManageWorkbench (1n << 1n)
    workspaceName: 'W',
    refetch: vi.fn(),
  }),
}));

const { useBid, useChecklist } = vi.hoisted(() => ({ useBid: vi.fn(), useChecklist: vi.fn() }));
vi.mock('~/features/workbench/hooks', () => ({ useBid, useChecklist }));

// The scheda gara reads three more sources. They are stubbed to "nothing known"
// so this test stays about the page's composition, not about its data.
const EMPTY = { data: null, loading: false, error: null, refetch: vi.fn() };
vi.mock('~/features/company/hooks', () => ({ useEligibility: () => EMPTY }));
vi.mock('~/features/tenders/hooks', () => ({
  useTenderAvailability: () => EMPTY,
  useTenderDetail: () => EMPTY,
}));
// The repair chat mounts the whole assistant stack; the page only owns whether
// it is composed in at all.
vi.mock('~/features/workbench/components/organisms/bid-repair-chat', () => ({
  BidRepairChat: () => <div data-testid="bid-repair-chat" />,
}));
// The DGUE preview is a live read behind a flag. Stubbed to a composed document
// with nothing open, so the flag-on test is about which card the page mounts,
// not about what the server would compute.
const { preview } = vi.hoisted(() => ({
  preview: {
    data: {
      leaves: [],
      gaps: [],
      declarations: undefined,
      ready: true,
      readyCount: 34,
      missingCount: 0,
      requestKnown: false,
      composedAt: '2026-09-04T10:00:00Z',
    },
  },
}));
vi.mock('~/lib/api/client', () => ({
  bidClient: {},
  companyClient: {},
  tenderClient: {},
  espdClient: {
    getResponsePreview: vi.fn(async () => preview.data),
    listExports: vi.fn(async () => ({ exports: [] })),
  },
}));

import { BidDetailPage } from './index';

const BASE_BID = {
  id: 'b1',
  tenderId: '42',
  tenderAvailable: true,
  tenderTitle: 'Road works',
  tenderBuyerName: 'City',
  goNoGo: 'undecided',
  stage: 'shortlisted',
  outcome: '',
  needsProfile: false,
  fitTier: '',
  reason: '',
  checklistDone: 0,
  checklistTotal: 0,
  tenderDeadline: '',
};

beforeEach(() => {
  vi.clearAllMocks();
  flags.dgue = false;
  useChecklist.mockReturnValue({ data: [], loading: false, error: null, refetch: vi.fn() });
});

describe('BidDetailPage', () => {
  it('renders a tombstone when the tender is unavailable', () => {
    useBid.mockReturnValue({
      data: { ...BASE_BID, tenderAvailable: false, tenderTitle: '', tenderBuyerName: '' },
      loading: false,
      error: null,
      refetch: vi.fn(),
    });
    render(<BidDetailPage />);
    expect(screen.getByText('bid.tombstone.title')).toBeTruthy();
  });

  it('shows go/no-go controls for a manager', () => {
    useBid.mockReturnValue({
      data: { ...BASE_BID, needsProfile: true },
      loading: false,
      error: null,
      refetch: vi.fn(),
    });
    render(<BidDetailPage />);
    expect(screen.getByRole('button', { name: 'bid.actions.markGo' })).toBeTruthy();
  });

  // The section order is the product rule; the page must compose the template,
  // not re-order blocks itself.
  it('renders the coverage, decision, checklist and stage sections in order', () => {
    useBid.mockReturnValue({ data: BASE_BID, loading: false, error: null, refetch: vi.fn() });
    const { container } = render(<BidDetailPage />);
    const labels = [...container.querySelectorAll('section')].map((s) =>
      s.getAttribute('aria-label'),
    );
    expect(labels).toEqual(['Decision', 'ESPD / DGUE checklist', 'Stage', 'Ask about this gara']);
  });

  // The flag decides which card occupies the checklist slot, and nothing else
  // about the page. Off — including while the flags are still loading — the
  // familiar checklist stays; on, the readiness card takes the same slot in the
  // same position, so the section order above holds either way.
  it('keeps the checklist while the dgue flag is off or still loading', async () => {
    useBid.mockReturnValue({ data: BASE_BID, loading: false, error: null, refetch: vi.fn() });
    for (const value of [false, undefined] as const) {
      flags.dgue = value;
      const { container, unmount } = await act(async () => render(<BidDetailPage />));
      expect(screen.queryByText('Your DGUE')).toBeNull();
      expect(
        [...container.querySelectorAll('section')].map((s) => s.getAttribute('aria-label')),
      ).toContain('ESPD / DGUE checklist');
      unmount();
    }
  });

  it('replaces the checklist with the readiness card when the dgue flag is on', async () => {
    flags.dgue = true;
    useBid.mockReturnValue({ data: BASE_BID, loading: false, error: null, refetch: vi.fn() });
    const { container } = await act(async () => render(<BidDetailPage />));
    expect(screen.getByText('Your DGUE')).toBeTruthy();
    // Same slot, same order — only the card inside it changed.
    expect(
      [...container.querySelectorAll('section')].map((s) => s.getAttribute('aria-label')),
    ).toEqual(['Decision', 'ESPD / DGUE checklist', 'Stage', 'Ask about this gara']);
  });

  it('offers no outcome section until the bid is a go', () => {
    useBid.mockReturnValue({ data: BASE_BID, loading: false, error: null, refetch: vi.fn() });
    const { container, unmount } = render(<BidDetailPage />);
    expect(
      [...container.querySelectorAll('section')].some(
        (s) => s.getAttribute('aria-label') === 'Outcome',
      ),
    ).toBe(false);
    unmount();

    useBid.mockReturnValue({
      data: { ...BASE_BID, goNoGo: 'go' },
      loading: false,
      error: null,
      refetch: vi.fn(),
    });
    const decided = render(<BidDetailPage />);
    expect(
      [...decided.container.querySelectorAll('section')].some(
        (s) => s.getAttribute('aria-label') === 'Outcome',
      ),
    ).toBe(true);
  });
});
