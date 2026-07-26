import { render, screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, opt?: unknown) =>
      typeof opt === 'string' ? opt : ((opt as { defaultValue?: string })?.defaultValue ?? key),
    i18n: { language: 'en-ie' },
  }),
}));
vi.mock('posthog-js/react', () => ({ usePostHog: () => ({ capture: vi.fn() }) }));
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
vi.mock('~/lib/api/client', () => ({ bidClient: {} }));

import { BidDetailPage } from './index';

describe('BidDetailPage', () => {
  it('renders a tombstone when the tender is unavailable', () => {
    useBid.mockReturnValue({
      data: {
        id: 'b1',
        tenderAvailable: false,
        goNoGo: 'undecided',
        stage: 'shortlisted',
        outcome: '',
        needsProfile: false,
        fitTier: '',
        checklistDone: 0,
        checklistTotal: 0,
        tenderDeadline: '',
      },
      loading: false,
      error: null,
      refetch: vi.fn(),
    });
    useChecklist.mockReturnValue({ data: [], loading: false, error: null, refetch: vi.fn() });
    render(<BidDetailPage />);
    expect(screen.getByText('bid.tombstone.title')).toBeTruthy();
  });

  it('shows go/no-go controls for a manager', () => {
    useBid.mockReturnValue({
      data: {
        id: 'b1',
        tenderAvailable: true,
        tenderTitle: 'Road works',
        tenderBuyerName: 'City',
        goNoGo: 'undecided',
        stage: 'shortlisted',
        outcome: '',
        needsProfile: true,
        fitTier: '',
        checklistDone: 0,
        checklistTotal: 0,
        tenderDeadline: '',
      },
      loading: false,
      error: null,
      refetch: vi.fn(),
    });
    useChecklist.mockReturnValue({ data: [], loading: false, error: null, refetch: vi.fn() });
    render(<BidDetailPage />);
    expect(screen.getByRole('button', { name: 'bid.actions.markGo' })).toBeTruthy();
  });
});
