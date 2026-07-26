import { fireEvent, render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (k: string, d?: string) => d ?? k, i18n: { language: 'en-ie' } }),
}));
const { useWorkbenches } = vi.hoisted(() => ({ useWorkbenches: vi.fn() }));
vi.mock('~/features/workbench/hooks', () => ({ useWorkbenches }));
const { recentState } = vi.hoisted(() => ({
  recentState: { items: [] as { workbenchId: string; workspaceId: string }[] },
}));
vi.mock('~/store/recent-workbenches', () => ({
  useRecentWorkbenchesStore: (sel: (s: { items: unknown[] }) => unknown) => sel(recentState),
}));

import { WorkbenchPicker } from './index';

describe('WorkbenchPicker', () => {
  beforeEach(() => {
    recentState.items = [];
  });

  it('calls onSelect with the chosen workbench id', () => {
    useWorkbenches.mockReturnValue({
      data: [{ id: 'wb1', name: 'Q3 Bids', description: '', visibility: 'private' }],
      loading: false,
      error: null,
      refetch: vi.fn(),
    });
    const onSelect = vi.fn();
    render(<WorkbenchPicker workspaceId="ws1" onSelect={onSelect} />);
    fireEvent.click(screen.getByRole('button', { name: /Q3 Bids/ }));
    expect(onSelect).toHaveBeenCalledWith('wb1');
  });

  it('orders the most-recently-visited workbench first', () => {
    recentState.items = [{ workbenchId: 'wb3', workspaceId: 'ws1' }];
    useWorkbenches.mockReturnValue({
      data: [
        { id: 'wb1', name: 'Alpha', description: '', visibility: 'private' },
        { id: 'wb2', name: 'Beta', description: '', visibility: 'private' },
        { id: 'wb3', name: 'Gamma', description: '', visibility: 'private' },
      ],
      loading: false,
      error: null,
      refetch: vi.fn(),
    });
    render(<WorkbenchPicker workspaceId="ws1" onSelect={vi.fn()} />);
    const buttons = screen.getAllByRole('button');
    expect(buttons.map((b) => b.textContent)).toEqual([
      expect.stringContaining('Gamma'),
      expect.stringContaining('Alpha'),
      expect.stringContaining('Beta'),
    ]);
  });
});
