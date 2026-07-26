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
vi.mock('@tanstack/react-router', () => ({
  Link: ({ children }: { children: ReactNode }) => <a href="/">{children}</a>,
}));
vi.mock('~/features/workbench/context', () => ({
  useWorkbenchContext: () => ({
    workbenchId: 'wb1',
    workbench: { id: 'wb1', workspaceId: 'ws1' },
    myPermissions: 2n,
    workspaceName: 'W',
    refetch: vi.fn(),
  }),
}));
const { useBids } = vi.hoisted(() => ({ useBids: vi.fn() }));
vi.mock('~/features/workbench/hooks', () => ({ useBids }));
vi.mock('~/features/account/components/pages/explore/use-client-shortlist', () => ({
  useClientShortlist: () => ({
    results: [],
    needsProfile: false,
    loading: false,
    error: null,
    refetch: vi.fn(),
  }),
}));
vi.mock('~/features/tenders', () => ({ useTenderLink: () => (_id: string, c: ReactNode) => c }));
vi.mock('~/features/account/components/organisms', () => ({ TenderResultCard: () => <div /> }));
vi.mock('~/features/workbench/components/organisms/workbench-chat-panel', () => ({
  WorkbenchChatPanel: () => <div data-testid="workbench-chat-panel" />,
}));

import { WorkbenchOverviewPage } from './index';

describe('WorkbenchOverviewPage', () => {
  it('surfaces a needs-you-now band for an active bid with a near deadline and incomplete checklist', () => {
    const soon = new Date(Date.now() + 5 * 86_400_000).toISOString();
    useBids.mockReturnValue({
      data: [
        {
          id: 'b1',
          tenderAvailable: true,
          tenderTitle: 'Bridge repair',
          tenderBuyerName: 'City',
          tenderDeadline: soon,
          goNoGo: 'go',
          stage: 'preparing',
          outcome: '',
          needsProfile: false,
          fitTier: 'strong',
          checklistDone: 1,
          checklistTotal: 5,
        },
      ],
      loading: false,
      error: null,
      refetch: vi.fn(),
    });
    render(<WorkbenchOverviewPage />);
    expect(screen.getByText('bid.bands.needsYouNow')).toBeTruthy();
  });
});
