import type { TenderFilters, TenderResult } from '@tendersbay/proto/tender/v1/tender_pb';
import { useCallback, useRef, useState } from 'react';
import { tenderClient } from '~/lib/api/client';

const PAGE_SIZE = 20;

/**
 * The tender filters the explore UI can set — a subset of the proto `TenderFilters`
 * message. Only fields set to a non-empty value constrain the search (an empty or
 * omitted field means "no constraint"); `search` takes these and `loadMore` reuses
 * the last set so paging stays within the same filtered result set.
 */
export type TenderFilterValues = Partial<
  Pick<TenderFilters, 'country' | 'cpv' | 'status' | 'deadlineFrom' | 'deadlineTo'>
>;

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
  search: (query: string, filters?: TenderFilterValues, workspaceId?: string) => Promise<void>;
  loadMore: () => Promise<void>;
} {
  const [results, setResults] = useState<TenderResult[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const queryRef = useRef('');
  const filtersRef = useRef<TenderFilterValues | undefined>(undefined);
  const workspaceIdRef = useRef<string | undefined>(undefined);
  const offsetRef = useRef(0);
  const requestIdRef = useRef(0);
  const inFlightRef = useRef(false);

  const search = useCallback(
    async (query: string, filters?: TenderFilterValues, workspaceId?: string) => {
      const requestId = ++requestIdRef.current;
      inFlightRef.current = true;
      queryRef.current = query;
      filtersRef.current = filters;
      workspaceIdRef.current = workspaceId;
      offsetRef.current = 0;
      setLoading(true);
      setError(null);

      try {
        const res = await tenderClient.searchTenders({
          query,
          filters,
          limit: PAGE_SIZE,
          offset: 0,
          workspaceId: workspaceId ?? '',
        });
        if (requestIdRef.current !== requestId) return;
        // Page by rows actually received, not by PAGE_SIZE: the server clamps
        // `limit` to the caller's auth tier (anon < 20), so a page can be short.
        offsetRef.current = res.results.length;
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
    },
    [],
  );

  const loadMore = useCallback(async () => {
    // A loadMore racing an in-flight request would page from a stale offset
    // and could win the id race against a pending search, dropping its
    // page-1 results — no-op instead (a search may still preempt loadMore).
    if (inFlightRef.current) return;
    const requestId = ++requestIdRef.current;
    inFlightRef.current = true;
    // Offset is the count of rows already held, not page*PAGE_SIZE — the server
    // may return fewer than PAGE_SIZE rows per page (tier clamp), so a fixed
    // stride would skip the rows between the clamp and PAGE_SIZE.
    const nextOffset = offsetRef.current;
    setLoading(true);

    try {
      const res = await tenderClient.searchTenders({
        query: queryRef.current,
        filters: filtersRef.current,
        limit: PAGE_SIZE,
        offset: nextOffset,
        workspaceId: workspaceIdRef.current ?? '',
      });
      if (requestIdRef.current !== requestId) return;
      offsetRef.current = nextOffset + res.results.length;
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
