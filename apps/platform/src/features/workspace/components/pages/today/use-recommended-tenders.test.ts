import { renderHook, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const searchTenders = vi.fn();
vi.mock('~/lib/api/client', () => ({
  tenderClient: { searchTenders: (...args: unknown[]) => searchTenders(...args) },
}));

import { useRecommendedTenders } from './use-recommended-tenders';

type FakeResult = { id: string; title: string };

function fakeResult(id: string): FakeResult {
  return { id, title: `Tender ${id}` };
}

describe('useRecommendedTenders', () => {
  beforeEach(() => {
    searchTenders.mockReset();
  });

  it('fetches the top open tenders once on mount and exposes them', async () => {
    searchTenders.mockResolvedValue({
      results: [fakeResult('1'), fakeResult('2'), fakeResult('3'), fakeResult('4')],
      hasMore: false,
    });

    const { result } = renderHook(() => useRecommendedTenders());
    expect(result.current.loading).toBe(true);

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(searchTenders).toHaveBeenCalledTimes(1);
    expect(searchTenders).toHaveBeenCalledWith({
      query: '',
      filters: { status: 'open' },
      limit: 4,
    });
    expect(result.current.tenders).toEqual([
      fakeResult('1'),
      fakeResult('2'),
      fakeResult('3'),
      fakeResult('4'),
    ]);
    expect(result.current.error).toBeNull();
  });

  it('degrades to an empty list and records the message when the search rejects', async () => {
    searchTenders.mockRejectedValue(new Error('network down'));

    const { result } = renderHook(() => useRecommendedTenders());

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.tenders).toEqual([]);
    expect(result.current.error).toBe('network down');
  });
});
