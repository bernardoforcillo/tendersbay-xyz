import { render, screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const { navigateMock, recommendedMock } = vi.hoisted(() => ({
  navigateMock: vi.fn(),
  recommendedMock: vi.fn(),
}));
vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => navigateMock,
  useParams: () => ({}),
}));
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (
      key: string,
      arg?: string | { defaultValue?: string; name?: string; count?: number; client?: string },
    ) => {
      const dv = typeof arg === 'string' ? arg : (arg?.defaultValue ?? key);
      if (typeof arg !== 'object' || arg === null) return dv;
      return dv
        .replace('{{name}}', arg.name ?? '')
        .replace('{{count}}', String(arg.count ?? ''))
        .replace('{{client}}', arg.client ?? '');
    },
    i18n: { language: 'en-ie' },
  }),
}));
vi.mock('~/features/workspace/context', () => ({
  useWorkspaceContext: () => ({ workspace: { id: 'ws-1', name: 'ACME' } }),
}));
vi.mock('~/features/account/hooks/use-workspace-chats', () => ({
  useWorkspaceChats: () => ({
    data: [
      { id: 'c1', title: 'Bandi cloud Lombardia', updatedAt: '2026-07-12T08:00:00Z' },
      { id: 'c2', title: '', updatedAt: '2026-07-11T08:00:00Z' },
    ],
    loading: false,
    error: null,
    refetch: vi.fn(),
  }),
}));
vi.mock('~/features/account/components/organisms', () => ({
  PageHeader: () => <header data-testid="page-header" />,
  SearchDock: ({ onPress }: { onPress?: () => void }) => (
    <button type="button" data-testid="search-dock" onClick={onPress} />
  ),
}));
// FirstRunProfile has its own dedicated test suite (its own GetClientProfile check,
// Skip, and carry-over pre-fill); Today's tests only need it to pass its content through.
vi.mock('~/features/workspace/components/organisms/first-run-profile', () => ({
  FirstRunProfile: ({ children }: { children: ReactNode }) => <>{children}</>,
}));
vi.mock('./use-recommended-tenders', () => ({
  useRecommendedTenders: () => recommendedMock(),
}));

import { useChatStore } from '~/store/chat';
import { WorkspaceTodayPage } from './index';

describe('WorkspaceTodayPage', () => {
  beforeEach(() => {
    navigateMock.mockReset();
    recommendedMock.mockReset();
    recommendedMock.mockReturnValue({ count: 0, needsProfile: false, loading: false, error: null });
  });

  it('greets, lists resumable chats, teaches Explore, keeps the dock', () => {
    render(<WorkspaceTodayPage />);
    expect(screen.getByRole('heading', { level: 1 }).textContent).toMatch(/Good/);
    expect(screen.getByText('Bandi cloud Lombardia')).toBeInTheDocument();
    expect(screen.getByText('Untitled conversation')).toBeInTheDocument();
    expect(screen.getByText('Find your next tender')).toBeInTheDocument();
    expect(screen.getByTestId('search-dock')).toBeInTheDocument();
  });

  it('resuming a chat clears the previous chat state and navigates to Explore', async () => {
    const { default: userEvent } = await import('@testing-library/user-event');
    const user = userEvent.setup();
    useChatStore.setState({
      messages: [{ id: 'old', role: 'user', content: 'stale', createdAt: '' }],
      pendingChoice: { id: 'p', question: 'q', options: [], allowCustom: false },
    });
    render(<WorkspaceTodayPage />);
    await user.click(screen.getByText('Bandi cloud Lombardia'));
    expect(useChatStore.getState().messages).toEqual([]);
    expect(useChatStore.getState().pendingChoice).toBeNull();
    expect(useChatStore.getState().currentChatId).toBe('c1');
    expect(navigateMock).toHaveBeenCalledWith({ to: '/explore' });
  });

  it('pressing the search dock navigates to Explore', async () => {
    const { default: userEvent } = await import('@testing-library/user-event');
    const user = userEvent.setup();
    render(<WorkspaceTodayPage />);
    await user.click(screen.getByTestId('search-dock'));
    expect(navigateMock).toHaveBeenCalledWith({ to: '/explore' });
  });

  it('shows the best-fit count nudge and the see-all link, hiding the Explore teaser', () => {
    recommendedMock.mockReturnValue({ count: 2, needsProfile: false, loading: false, error: null });
    render(<WorkspaceTodayPage />);
    expect(screen.getByText('2 best-fit tenders ready for ACME')).toBeInTheDocument();
    expect(screen.getByText('All in Explore →')).toBeInTheDocument();
    expect(screen.queryByText('Find your next tender')).not.toBeInTheDocument();
  });

  it('falls back to the Explore teaser and hides the recommended nudge with no tenders', () => {
    recommendedMock.mockReturnValue({ count: 0, needsProfile: false, loading: false, error: null });
    render(<WorkspaceTodayPage />);
    expect(screen.getByText('Find your next tender')).toBeInTheDocument();
    expect(screen.queryByText('All in Explore →')).not.toBeInTheDocument();
  });

  it('pressing see-all navigates to Explore', async () => {
    recommendedMock.mockReturnValue({ count: 1, needsProfile: false, loading: false, error: null });
    const { default: userEvent } = await import('@testing-library/user-event');
    const user = userEvent.setup();
    render(<WorkspaceTodayPage />);
    await user.click(screen.getByText('All in Explore →'));
    expect(navigateMock).toHaveBeenCalledWith({ to: '/explore' });
  });
});
