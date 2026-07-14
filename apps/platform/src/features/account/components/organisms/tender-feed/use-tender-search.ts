import type { TenderResult } from '@tendersbay/proto/tender/v1/tender_pb';
import { useCallback, useRef, useState } from 'react';
import { tenderClient } from '~/lib/api/client';

const PAGE_SIZE = 20;

function errorMessage(e: unknown): string {
  return e instanceof Error ? e.message : 'Something went wrong';
}

/**
 * Feed-scoped search hook for the tender-feed organism: wraps
 * `tenderClient.searchTenders` with paging (search replaces, loadMore
 * appends) and a request-id guard so an in-flight response from a
 * superseded call can never clobber a newer one.
 */
export function useTenderSearch(): {
  results: TenderResult[];
  hasMore: boolean;
  loading: boolean;
  error: string | null;
  search: (query: string) => Promise<void>;
  loadMore: () => Promise<void>;
} {
  const [results, setResults] = useState<TenderResult[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const queryRef = useRef('');
  const offsetRef = useRef(0);
  const requestIdRef = useRef(0);
  const inFlightRef = useRef(false);

  const search = useCallback(async (query: string) => {
    const requestId = ++requestIdRef.current;
    inFlightRef.current = true;
    queryRef.current = query;
    offsetRef.current = 0;
    setLoading(true);
    setError(null);

    try {
      const res = await tenderClient.searchTenders({ query, limit: PAGE_SIZE, offset: 0 });
      if (requestIdRef.current !== requestId) return;
      setResults(res.results);
      setHasMore(res.hasMore);
      setError(null);
    } catch (e: unknown) {
      if (requestIdRef.current !== requestId) return;
      setError(errorMessage(e));
    } finally {
      if (requestIdRef.current === requestId) {
        inFlightRef.current = false;
        setLoading(false);
      }
    }
  }, []);

  const loadMore = useCallback(async () => {
    // A loadMore racing an in-flight request would page from a stale offset
    // and could win the id race against a pending search, dropping its
    // page-1 results — no-op instead (a search may still preempt loadMore).
    if (inFlightRef.current) return;
    const requestId = ++requestIdRef.current;
    inFlightRef.current = true;
    const nextOffset = offsetRef.current + PAGE_SIZE;
    setLoading(true);

    try {
      const res = await tenderClient.searchTenders({
        query: queryRef.current,
        limit: PAGE_SIZE,
        offset: nextOffset,
      });
      if (requestIdRef.current !== requestId) return;
      offsetRef.current = nextOffset;
      setResults((prev) => [...prev, ...res.results]);
      setHasMore(res.hasMore);
      setError(null);
    } catch (e: unknown) {
      if (requestIdRef.current !== requestId) return;
      setError(errorMessage(e));
    } finally {
      if (requestIdRef.current === requestId) {
        inFlightRef.current = false;
        setLoading(false);
      }
    }
  }, []);

  return { results, hasMore, loading, error, search, loadMore };
}
