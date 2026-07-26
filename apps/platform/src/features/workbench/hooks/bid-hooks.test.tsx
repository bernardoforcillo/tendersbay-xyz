import { renderHook, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const { listBids, getBid, listChecklistItems } = vi.hoisted(() => ({
  listBids: vi.fn(),
  getBid: vi.fn(),
  listChecklistItems: vi.fn(),
}));
vi.mock('~/lib/api/client', () => ({
  bidClient: { listBids, getBid, listChecklistItems },
}));

import { WorkbenchContext } from '~/features/workbench/context';
import { useBid, useBids, useChecklist } from './index';

function wrapper({ children }: { children: ReactNode }) {
  return (
    <WorkbenchContext.Provider
      value={{
        workbenchId: 'wb-1',
        // biome-ignore lint/suspicious/noExplicitAny: minimal ctx for the hook under test
        workbench: {} as any,
        myPermissions: 0n,
        workspaceName: 'W',
        refetch: () => {},
      }}
    >
      {children}
    </WorkbenchContext.Provider>
  );
}

describe('bid hooks', () => {
  beforeEach(() => {
    listBids.mockReset();
    getBid.mockReset();
    listChecklistItems.mockReset();
  });

  it('useBids lists bids for the given workbench', async () => {
    listBids.mockResolvedValue({ bids: [{ id: 'b1' }] });
    const { result } = renderHook(() => useBids('wb-1'));
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(listBids).toHaveBeenCalledWith({ workbenchId: 'wb-1' });
    expect(result.current.data).toEqual([{ id: 'b1' }]);
  });

  it('useBid reads workbenchId from context', async () => {
    getBid.mockResolvedValue({ bid: { id: 'b1' } });
    const { result } = renderHook(() => useBid('b1'), { wrapper });
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(getBid).toHaveBeenCalledWith({ workbenchId: 'wb-1', bidId: 'b1' });
    expect(result.current.data).toEqual({ id: 'b1' });
  });

  it('useChecklist reads workbenchId from context', async () => {
    listChecklistItems.mockResolvedValue({ items: [{ id: 'c1' }] });
    const { result } = renderHook(() => useChecklist('b1'), { wrapper });
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(listChecklistItems).toHaveBeenCalledWith({ workbenchId: 'wb-1', bidId: 'b1' });
    expect(result.current.data).toEqual([{ id: 'c1' }]);
  });
});
