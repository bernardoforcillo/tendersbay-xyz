import { act, renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const searchTenders = vi.fn();
vi.mock('~/lib/api/client', () => ({
  tenderClient: { searchTenders: (...args: unknown[]) => searchTenders(...args) },
}));

import { useTenderSearch } from './use-tender-search';

type FakeResult = { id: string; title: string };

function fakeResult(id: string): FakeResult {
  return { id, title: `Tender ${id}` };
}

function createDeferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

describe('useTenderSearch', () => {
  beforeEach(() => {
    searchTenders.mockReset();
  });

  it('replaces results with the first page and exposes hasMore', async () => {
    searchTenders.mockResolvedValue({ results: [fakeResult('1'), fakeResult('2')], hasMore: true });

    const { result } = renderHook(() => useTenderSearch());
    await act(async () => {
      await result.current.search('roads');
    });

    expect(searchTenders).toHaveBeenCalledWith({ query: 'roads', limit: 20, offset: 0 });
    expect(result.current.results).toEqual([fakeResult('1'), fakeResult('2')]);
    expect(result.current.hasMore).toBe(true);
    expect(result.current.loading).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('loadMore appends the next page at offset 20 for the same query', async () => {
    searchTenders.mockResolvedValueOnce({ results: [fakeResult('1')], hasMore: true });
    const { result } = renderHook(() => useTenderSearch());
    await act(async () => {
      await result.current.search('roads');
    });

    searchTenders.mockResolvedValueOnce({ results: [fakeResult('2')], hasMore: false });
    await act(async () => {
      await result.current.loadMore();
    });

    expect(searchTenders).toHaveBeenLastCalledWith({ query: 'roads', limit: 20, offset: 20 });
    expect(result.current.results).toEqual([fakeResult('1'), fakeResult('2')]);
    expect(result.current.hasMore).toBe(false);
  });

  it('a new search resets the offset and replaces results instead of appending', async () => {
    searchTenders.mockResolvedValueOnce({ results: [fakeResult('1')], hasMore: true });
    const { result } = renderHook(() => useTenderSearch());
    await act(async () => {
      await result.current.search('roads');
    });

    searchTenders.mockResolvedValueOnce({ results: [fakeResult('2')], hasMore: false });
    await act(async () => {
      await result.current.loadMore();
    });
    expect(result.current.results).toHaveLength(2);

    searchTenders.mockResolvedValueOnce({ results: [fakeResult('9')], hasMore: false });
    await act(async () => {
      await result.current.search('bridges');
    });

    expect(searchTenders).toHaveBeenLastCalledWith({ query: 'bridges', limit: 20, offset: 0 });
    expect(result.current.results).toEqual([fakeResult('9')]);
  });

  it('applies only the latest request when an earlier search resolves after a later one', async () => {
    const deferredA = createDeferred<{ results: FakeResult[]; hasMore: boolean }>();
    const deferredB = createDeferred<{ results: FakeResult[]; hasMore: boolean }>();
    searchTenders.mockImplementationOnce(() => deferredA.promise);
    searchTenders.mockImplementationOnce(() => deferredB.promise);

    const { result } = renderHook(() => useTenderSearch());

    let promiseA!: Promise<void>;
    let promiseB!: Promise<void>;
    act(() => {
      promiseA = result.current.search('a');
    });
    act(() => {
      promiseB = result.current.search('b');
    });

    await act(async () => {
      deferredB.resolve({ results: [fakeResult('b-1')], hasMore: false });
      await promiseB;
    });
    expect(result.current.results).toEqual([fakeResult('b-1')]);

    await act(async () => {
      deferredA.resolve({ results: [fakeResult('a-1')], hasMore: true });
      await promiseA;
    });

    expect(result.current.results).toEqual([fakeResult('b-1')]);
    expect(result.current.hasMore).toBe(false);
  });

  it('ignores loadMore while a request is in flight, so a pending search keeps its page', async () => {
    const deferred = createDeferred<{ results: FakeResult[]; hasMore: boolean }>();
    searchTenders.mockImplementationOnce(() => deferred.promise);

    const { result } = renderHook(() => useTenderSearch());

    let searchPromise!: Promise<void>;
    act(() => {
      searchPromise = result.current.search('roads');
    });
    await act(async () => {
      await result.current.loadMore();
    });
    expect(searchTenders).toHaveBeenCalledTimes(1);

    await act(async () => {
      deferred.resolve({ results: [fakeResult('1')], hasMore: true });
      await searchPromise;
    });
    expect(result.current.results).toEqual([fakeResult('1')]);
    expect(result.current.hasMore).toBe(true);
  });

  it('sets an error message when search rejects', async () => {
    searchTenders.mockRejectedValueOnce(new Error('network down'));
    const { result } = renderHook(() => useTenderSearch());

    await act(async () => {
      await result.current.search('roads');
    });

    expect(result.current.error).toBe('network down');
    expect(result.current.loading).toBe(false);
  });

  it('keeps previous results and sets the error when loadMore rejects', async () => {
    searchTenders.mockResolvedValueOnce({ results: [fakeResult('1')], hasMore: true });
    const { result } = renderHook(() => useTenderSearch());
    await act(async () => {
      await result.current.search('roads');
    });

    searchTenders.mockRejectedValueOnce(new Error('timeout'));
    await act(async () => {
      await result.current.loadMore();
    });

    expect(result.current.results).toEqual([fakeResult('1')]);
    expect(result.current.error).toBe('timeout');
    expect(result.current.loading).toBe(false);
  });

  it('clears a previous error when a new search is issued', async () => {
    searchTenders.mockRejectedValueOnce(new Error('network down'));
    const { result } = renderHook(() => useTenderSearch());
    await act(async () => {
      await result.current.search('roads');
    });
    expect(result.current.error).toBe('network down');

    searchTenders.mockResolvedValueOnce({ results: [fakeResult('1')], hasMore: false });
    await act(async () => {
      await result.current.search('bridges');
    });

    expect(result.current.error).toBeNull();
    expect(result.current.results).toEqual([fakeResult('1')]);
  });
});
