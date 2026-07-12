import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

const { navigateMock } = vi.hoisted(() => ({ navigateMock: vi.fn() }));
vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => navigateMock,
  useParams: () => ({}),
}));
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, arg?: string | { defaultValue?: string; name?: string }) => {
      const dv = typeof arg === 'string' ? arg : (arg?.defaultValue ?? key);
      return typeof arg === 'object' && arg?.name ? dv.replace('{{name}}', arg.name) : dv;
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
  SearchDock: () => <div data-testid="search-dock" />,
}));

import { useChatStore } from '~/store/chat';
import { WorkspaceTodayPage } from './index';

describe('WorkspaceTodayPage', () => {
  it('greets, lists resumable chats, teaches Explore, keeps the dock', () => {
    render(<WorkspaceTodayPage />);
    expect(screen.getByRole('heading', { level: 1 }).textContent).toMatch(/Good/);
    expect(screen.getByText('Bandi cloud Lombardia')).toBeInTheDocument();
    expect(screen.getByText('Untitled conversation')).toBeInTheDocument();
    expect(screen.getByText('Find your next tender')).toBeInTheDocument();
    expect(screen.getByTestId('search-dock')).toBeInTheDocument();
  });

  it('resuming a chat sets it current and navigates to Explore', async () => {
    const { default: userEvent } = await import('@testing-library/user-event');
    const user = userEvent.setup();
    render(<WorkspaceTodayPage />);
    await user.click(screen.getByText('Bandi cloud Lombardia'));
    expect(useChatStore.getState().currentChatId).toBe('c1');
    expect(navigateMock).toHaveBeenCalledWith({ to: '/explore' });
  });
});
