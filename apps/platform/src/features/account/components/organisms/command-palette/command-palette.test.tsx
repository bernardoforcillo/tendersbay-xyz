import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useSidebarStore } from '~/store/sidebar';
import { useWorkspaceStore } from '~/store/workspace';

const { navigateMock } = vi.hoisted(() => ({ navigateMock: vi.fn() }));
vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => navigateMock,
  useParams: () => ({}),
}));
vi.mock('~/features/workspace/hooks', () => ({
  useMyWorkspaces: () => ({ data: [{ id: 'ws-2', name: 'ACME' }], loading: false }),
}));
// Return defaultValue strings without initializing the full i18n stack.
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, defaultValue?: string | { defaultValue?: string }) =>
      typeof defaultValue === 'string' ? defaultValue : (defaultValue?.defaultValue ?? key),
    i18n: { language: 'en-ie' },
  }),
}));

import { CommandPalette } from './index';

describe('CommandPalette', () => {
  beforeEach(() => {
    navigateMock.mockClear();
    useWorkspaceStore.setState({ currentWorkspaceId: 'ws-1' });
    useSidebarStore.getState().setPaletteOpen(true);
  });

  it('renders the search input and navigate commands when open', () => {
    render(<CommandPalette />);
    expect(screen.getByRole('searchbox')).toBeInTheDocument();
    expect(screen.getByText('Explore')).toBeInTheDocument();
    expect(screen.getByText('Workspace settings')).toBeInTheDocument();
  });

  it('filters commands as the user types', async () => {
    const user = userEvent.setup();
    render(<CommandPalette />);
    await user.type(screen.getByRole('searchbox'), 'expl');
    expect(screen.getByText('Explore')).toBeInTheDocument();
    expect(screen.queryByText('Workspace settings')).not.toBeInTheDocument();
  });

  it('navigates and closes on selection', async () => {
    const user = userEvent.setup();
    render(<CommandPalette />);
    await user.click(screen.getByText('Explore'));
    expect(navigateMock).toHaveBeenCalledWith({ to: '/explore', params: undefined });
    expect(useSidebarStore.getState().paletteOpen).toBe(false);
  });

  it('opens on Ctrl+K', async () => {
    useSidebarStore.getState().setPaletteOpen(false);
    const user = userEvent.setup();
    render(<CommandPalette />);
    await user.keyboard('{Control>}k{/Control}');
    expect(useSidebarStore.getState().paletteOpen).toBe(true);
  });

  it('closes and clears the query on Ctrl+K, and reopens with an empty input', async () => {
    const user = userEvent.setup();
    render(<CommandPalette />);
    await user.type(screen.getByRole('searchbox'), 'expl');
    expect(screen.getByRole('searchbox')).toHaveValue('expl');

    await user.keyboard('{Control>}k{/Control}');
    expect(useSidebarStore.getState().paletteOpen).toBe(false);

    await user.keyboard('{Control>}k{/Control}');
    expect(useSidebarStore.getState().paletteOpen).toBe(true);
    expect(screen.getByRole('searchbox')).toHaveValue('');
  });
});
