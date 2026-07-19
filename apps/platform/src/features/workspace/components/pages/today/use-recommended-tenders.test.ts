import { renderHook, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const recommendTendersForClient = vi.fn();
vi.mock('~/lib/api/client', () => ({
  tenderClient: {
    recommendTendersForClient: (...args: unknown[]) => recommendTendersForClient(...args),
  },
}));

import { useRecommendedTenders } from './use-recommended-tenders';

describe('useRecommendedTenders', () => {
  beforeEach(() => {
    recommendTendersForClient.mockReset();
  });

  it('fetches the client shortlist count for the given workspace', async () => {
    recommendTendersForClient.mockResolvedValue({
      results: [{ tender: { id: '1' } }, { tender: { id: '2' } }],
      needsProfile: false,
    });

    const { result } = renderHook(() => useRecommendedTenders('ws-1'));
    expect(result.current.loading).toBe(true);

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(recommendTendersForClient).toHaveBeenCalledWith({ workspaceId: 'ws-1', limit: 4 });
    expect(result.current.count).toBe(2);
    expect(result.current.needsProfile).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('surfaces needsProfile without treating it as an error', async () => {
    recommendTendersForClient.mockResolvedValue({ results: [], needsProfile: true });

    const { result } = renderHook(() => useRecommendedTenders('ws-1'));
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.needsProfile).toBe(true);
    expect(result.current.count).toBe(0);
    expect(result.current.error).toBeNull();
  });

  it('degrades to a zero count and records the message when the request rejects', async () => {
    recommendTendersForClient.mockRejectedValue(new Error('network down'));

    const { result } = renderHook(() => useRecommendedTenders('ws-1'));
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.count).toBe(0);
    expect(result.current.error).toBe('network down');
  });
});
