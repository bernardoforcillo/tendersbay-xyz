import type {
  FacetCount,
  TenderFilters,
  TenderResult,
} from '@tendersbay/proto/tender/v1/tender_pb';
import { useCallback, useRef, useState } from 'react';
import { tenderClient } from '~/lib/api/client';

const PAGE_SIZE = 20;

/**
 * The tender filters the explore UI can set — a subset of the proto `TenderFilters`
 * message. Only fields set to a non-empty value constrain the search (an empty
 * list or omitted field means "no constraint"); `search` takes these and
 * `loadMore` reuses the last set so paging stays within the same filtered result
 * set.
 *
 * The list fields are OR-ed within themselves and AND-ed with each other:
 * `{ countries: ['IT', 'DE'], cpvPrefixes: ['45'] }` reads as "construction in
 * Italy or Germany".
 */
export type TenderFilterValues = Partial<
  Pick<
    TenderFilters,
    | 'countries'
    | 'cpvPrefixes'
    | 'statuses'
    | 'nutsPrefixes'
    | 'buyer'
    | 'valueMin'
    | 'valueMax'
    | 'deadlineFrom'
    | 'deadlineTo'
  >
>;

/** Result orderings the UI can request. */
export type TenderSort = 'relevance' | 'deadline' | 'published' | 'value';

/**
 * Per-facet counts for the filter bar, plus what the server understood of the
 * query. `mode`/`degraded` say which retrievers actually answered, so a
 * degraded search can be shown as degraded rather than silently returning
 * something else; `appliedFilters`/`appliedQuery` say which constraints were
 * lifted out of the query text, so the user can see and undo them.
 */
export type SearchMeta = {
  mode: string;
  degraded: boolean;
  appliedFilters?: TenderFilters;
  appliedQuery: string;
  countryFacets: FacetCount[];
  statusFacets: FacetCount[];
  cpvDivisionFacets: FacetCount[];
};

const EMPTY_META: SearchMeta = {
  mode: '',
  degraded: false,
  appliedQuery: '',
  countryFacets: [],
  statusFacets: [],
  cpvDivisionFacets: [],
};

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
  meta: SearchMeta;
  search: (
    query: string,
    filters?: TenderFilterValues,
    workspaceId?: string,
    sort?: TenderSort,
  ) => Promise<void>;
  loadMore: () => Promise<void>;
} {
  const [results, setResults] = useState<TenderResult[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [meta, setMeta] = useState<SearchMeta>(EMPTY_META);

  const queryRef = useRef('');
  const filtersRef = useRef<TenderFilterValues | undefined>(undefined);
  const workspaceIdRef = useRef<string | undefined>(undefined);
  const sortRef = useRef<TenderSort>('relevance');
  const offsetRef = useRef(0);
  const requestIdRef = useRef(0);
  const inFlightRef = useRef(false);

  const search = useCallback(
    async (
      query: string,
      filters?: TenderFilterValues,
      workspaceId?: string,
      sort: TenderSort = 'relevance',
    ) => {
      const requestId = ++requestIdRef.current;
      inFlightRef.current = true;
      queryRef.current = query;
      filtersRef.current = filters;
      workspaceIdRef.current = workspaceId;
      sortRef.current = sort;
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
          sort,
        });
        if (requestIdRef.current !== requestId) return;
        // Page by rows actually received, not by PAGE_SIZE: the server clamps
        // `limit` to the caller's auth tier (anon < 20), so a page can be short.
        offsetRef.current = res.results.length;
        setResults(res.results);
        setHasMore(res.hasMore);
        setMeta(metaOf(res));
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
        sort: sortRef.current,
      });
      if (requestIdRef.current !== requestId) return;
      offsetRef.current = nextOffset + res.results.length;
      setResults((prev) => [...prev, ...res.results]);
      setHasMore(res.hasMore);
      setMeta(metaOf(res));
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

  return { results, hasMore, loading, error, meta, search, loadMore };
}

/**
 * Projects the response's metadata onto `SearchMeta`. Kept separate from the
 * request plumbing so `search` and `loadMore` cannot drift into reporting
 * different things about the same result set.
 */
function metaOf(res: {
  retrievalMode: string;
  degraded: boolean;
  appliedFilters?: TenderFilters;
  appliedQuery: string;
  countryFacets: FacetCount[];
  statusFacets: FacetCount[];
  cpvDivisionFacets: FacetCount[];
}): SearchMeta {
  return {
    mode: res.retrievalMode,
    degraded: res.degraded,
    appliedFilters: res.appliedFilters,
    appliedQuery: res.appliedQuery,
    countryFacets: res.countryFacets,
    statusFacets: res.statusFacets,
    cpvDivisionFacets: res.cpvDivisionFacets,
  };
}
