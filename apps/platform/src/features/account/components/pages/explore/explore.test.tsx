import type { ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';
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
  it('renders the chat window and no search input', () => {
    renderWithI18n(<AccountExplorePage />);
    expect(screen.getByTestId('chat-window')).toBeInTheDocument();
    expect(screen.queryByRole('textbox', { name: 'Search' })).not.toBeInTheDocument();
  });
});
