import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const searchTenders = vi.fn();
vi.mock('~/lib/api/client', () => ({
  tenderClient: { searchTenders: (...args: unknown[]) => searchTenders(...args) },
}));

import { useLandingSearch } from './use-landing-search';

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

/** Advance past the 300ms debounce and flush the resolved promise microtasks. */
async function flushDebounce(ms = 300) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });
}

describe('useLandingSearch', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    searchTenders.mockReset();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it('stays idle and does not call the API below the 2-char minimum', async () => {
    const { result } = renderHook((q: string) => useLandingSearch(q), { initialProps: 'r' });
    await flushDebounce();

    expect(result.current.status).toBe('idle');
    expect(result.current.results).toEqual([]);
    expect(searchTenders).not.toHaveBeenCalled();
  });

  it('debounces, then searches with limit 3 and offset 0', async () => {
    searchTenders.mockResolvedValue({
      results: [fakeResult('1'), fakeResult('2')],
      hasMore: false,
    });
    const { result } = renderHook((q: string) => useLandingSearch(q), { initialProps: 'roads' });

    // Nothing fires during the debounce window.
    expect(searchTenders).not.toHaveBeenCalled();

    await flushDebounce();

    expect(searchTenders).toHaveBeenCalledWith({ query: 'roads', limit: 3, offset: 0 });
    expect(result.current.status).toBe('results');
    expect(result.current.results).toHaveLength(2);
  });

  it('coalesces rapid keystrokes into a single request for the final query', async () => {
    searchTenders.mockResolvedValue({ results: [fakeResult('1')], hasMore: false });
    const { rerender } = renderHook((q: string) => useLandingSearch(q), { initialProps: 'r' });

    for (const q of ['ro', 'roa', 'road', 'roads']) {
      await act(async () => {
        await vi.advanceTimersByTimeAsync(100); // each shorter than the 300ms debounce
      });
      rerender(q);
    }
    await flushDebounce();

    expect(searchTenders).toHaveBeenCalledTimes(1);
    expect(searchTenders).toHaveBeenCalledWith({ query: 'roads', limit: 3, offset: 0 });
  });

  it('caps rendered results at 3 even if the server returns more', async () => {
    searchTenders.mockResolvedValue({
      results: Array.from({ length: 5 }, (_, i) => fakeResult(String(i))),
      hasMore: true,
    });
    const { result } = renderHook((q: string) => useLandingSearch(q), { initialProps: 'roads' });
    await flushDebounce();

    expect(result.current.results).toHaveLength(3);
    expect(result.current.status).toBe('results');
  });

  it('reports an empty state (no sample fallback) when the query returns nothing', async () => {
    searchTenders.mockResolvedValue({ results: [], hasMore: false });
    const { result } = renderHook((q: string) => useLandingSearch(q), { initialProps: 'zxqw' });
    await flushDebounce();

    expect(result.current.status).toBe('empty');
    expect(result.current.results).toEqual([]);
  });

  it('reports an error state when the search rejects', async () => {
    searchTenders.mockRejectedValue(new Error('network down'));
    const { result } = renderHook((q: string) => useLandingSearch(q), { initialProps: 'roads' });
    await flushDebounce();

    expect(result.current.status).toBe('error');
    expect(result.current.results).toEqual([]);
  });

  it('applies only the latest request when an earlier one resolves after a later one', async () => {
    const deferredA = createDeferred<{ results: FakeResult[]; hasMore: boolean }>();
    const deferredB = createDeferred<{ results: FakeResult[]; hasMore: boolean }>();
    searchTenders.mockImplementationOnce(() => deferredA.promise);
    searchTenders.mockImplementationOnce(() => deferredB.promise);

    const { result, rerender } = renderHook((q: string) => useLandingSearch(q), {
      initialProps: 'aa',
    });
    await flushDebounce(); // fires request A (pending)
    rerender('bb');
    await flushDebounce(); // fires request B (pending)

    await act(async () => {
      deferredB.resolve({ results: [fakeResult('b')], hasMore: false });
    });
    expect(result.current.results).toEqual([fakeResult('b')]);

    // The stale A response must be dropped by the request-id guard.
    await act(async () => {
      deferredA.resolve({ results: [fakeResult('a')], hasMore: true });
    });
    expect(result.current.results).toEqual([fakeResult('b')]);
    expect(result.current.status).toBe('results');
  });

  it('returns to idle and drops an in-flight response when the query falls below the minimum', async () => {
    const deferred = createDeferred<{ results: FakeResult[]; hasMore: boolean }>();
    searchTenders.mockImplementationOnce(() => deferred.promise);

    const { result, rerender } = renderHook((q: string) => useLandingSearch(q), {
      initialProps: 'roads',
    });
    await flushDebounce(); // request in flight
    expect(result.current.status).toBe('loading');

    rerender('r'); // below minimum
    expect(result.current.status).toBe('idle');

    // A late resolution of the abandoned request must not resurrect results.
    await act(async () => {
      deferred.resolve({ results: [fakeResult('1')], hasMore: false });
    });
    expect(result.current.status).toBe('idle');
    expect(result.current.results).toEqual([]);
  });

  it('fires onResolved once per resolved search with lengths, never the raw query', async () => {
    searchTenders.mockResolvedValue({
      results: [fakeResult('1'), fakeResult('2')],
      hasMore: false,
    });
    const onResolved = vi.fn();
    renderHook((q: string) => useLandingSearch(q, { onResolved }), { initialProps: 'roads' });
    await flushDebounce();

    expect(onResolved).toHaveBeenCalledTimes(1);
    expect(onResolved).toHaveBeenCalledWith({ queryLength: 5, resultCount: 2 });
  });

  it('does not fire onResolved on error', async () => {
    searchTenders.mockRejectedValue(new Error('boom'));
    const onResolved = vi.fn();
    renderHook((q: string) => useLandingSearch(q, { onResolved }), { initialProps: 'roads' });
    await flushDebounce();

    expect(onResolved).not.toHaveBeenCalled();
  });
});
