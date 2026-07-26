import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (k: string, d?: string) => d ?? k, i18n: { language: 'en-ie' } }),
}));
const { useWorkbenches } = vi.hoisted(() => ({ useWorkbenches: vi.fn() }));
vi.mock('~/features/workbench/hooks', () => ({ useWorkbenches }));
vi.mock('~/store/recent-workbenches', () => ({
  useRecentWorkbenchesStore: (sel: (s: { items: unknown[] }) => unknown) => sel({ items: [] }),
}));

import { WorkbenchPicker } from './index';

describe('WorkbenchPicker', () => {
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
});
