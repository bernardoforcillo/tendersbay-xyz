import { act, renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const recommendTendersForClient = vi.fn();
vi.mock('~/lib/api/client', () => ({
  tenderClient: {
    recommendTendersForClient: (...args: unknown[]) => recommendTendersForClient(...args),
  },
}));

import { useClientShortlist } from './use-client-shortlist';

function fakeResult(id: string) {
  return { tender: { id }, fitTier: 'strong', reason: {} };
}

describe('useClientShortlist', () => {
  beforeEach(() => {
    recommendTendersForClient.mockReset();
  });

  it('does not call the client when workspaceId is null', () => {
    renderHook(() => useClientShortlist(null));
    expect(recommendTendersForClient).not.toHaveBeenCalled();
  });

  it('fetches and exposes results plus needsProfile on a real workspaceId', async () => {
    recommendTendersForClient.mockResolvedValue({
      results: [fakeResult('1')],
      needsProfile: false,
    });

    const { result } = renderHook(() => useClientShortlist('ws-1'));
    await act(async () => {
      await Promise.resolve();
    });

    expect(recommendTendersForClient).toHaveBeenCalledWith({ workspaceId: 'ws-1', limit: 3 });
    expect(result.current.results).toEqual([fakeResult('1')]);
    expect(result.current.needsProfile).toBe(false);
    expect(result.current.loading).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('surfaces needsProfile:true with an empty results array', async () => {
    recommendTendersForClient.mockResolvedValue({ results: [], needsProfile: true });

    const { result } = renderHook(() => useClientShortlist('ws-1'));
    await act(async () => {
      await Promise.resolve();
    });

    expect(result.current.needsProfile).toBe(true);
    expect(result.current.results).toEqual([]);
  });

  it('degrades to an empty list on error, without throwing', async () => {
    recommendTendersForClient.mockRejectedValue(new Error('network down'));

    const { result } = renderHook(() => useClientShortlist('ws-1'));
    await act(async () => {
      await Promise.resolve();
    });

    expect(result.current.error).toBe('network down');
    expect(result.current.results).toEqual([]);
  });

  it('re-fetches when workspaceId changes', async () => {
    recommendTendersForClient.mockResolvedValue({
      results: [fakeResult('1')],
      needsProfile: false,
    });

    const { rerender } = renderHook(({ id }) => useClientShortlist(id), {
      initialProps: { id: 'ws-1' },
    });
    await act(async () => {
      await Promise.resolve();
    });
    rerender({ id: 'ws-2' });
    await act(async () => {
      await Promise.resolve();
    });

    expect(recommendTendersForClient).toHaveBeenCalledTimes(2);
    expect(recommendTendersForClient).toHaveBeenLastCalledWith({ workspaceId: 'ws-2', limit: 3 });
  });
});
