import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useChatStore } from '~/store/chat';
import { renderWithI18n } from '~/test/utils';

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

import { screen } from '@testing-library/react';
import { AccountExplorePage } from './index';

describe('AccountExplorePage', () => {
  beforeEach(() => {
    useChatStore.getState().reset();
  });

  it('renders the chat window and no search input', () => {
    renderWithI18n(<AccountExplorePage />);
    expect(screen.getByTestId('chat-window')).toBeInTheDocument();
    expect(screen.queryByRole('textbox', { name: 'Search' })).not.toBeInTheDocument();
  });

  it('resets a workbench-scoped chat to the workspace scope on mount, so the explore ChatWindow does not hydrate the workbench chat', () => {
    // Simulate arriving at /explore right after visiting a workbench: the
    // store is still scoped to that workbench (persisted to sessionStorage).
    useChatStore.setState({ currentChatId: 'chat-workbench', currentWorkbenchId: 'wb-1' });

    renderWithI18n(<AccountExplorePage />);

    expect(useChatStore.getState().currentChatId).toBeNull();
    expect(useChatStore.getState().currentWorkbenchId).toBeNull();
  });

  it('leaves an already workspace-scoped chat untouched on mount', () => {
    useChatStore.setState({ currentChatId: 'chat-explore', currentWorkbenchId: null });

    renderWithI18n(<AccountExplorePage />);

    expect(useChatStore.getState().currentChatId).toBe('chat-explore');
    expect(useChatStore.getState().currentWorkbenchId).toBeNull();
  });
});
