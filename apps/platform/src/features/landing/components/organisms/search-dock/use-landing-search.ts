import type { TenderResult } from '@tendersbay/proto/tender/v1/tender_pb';
import { useEffect, useRef, useState } from 'react';
import { tenderClient } from '~/lib/api/client';

/** Debounce before a keystroke turns into a request — one search per pause, not per key. */
const DEBOUNCE_MS = 300;
/** Below this trimmed length we stay idle: a 1-char query is noise, not intent. */
const MIN_QUERY_LENGTH = 2;
/** Hero teaser shows at most three cards; the server also clamps anon callers to its tier. */
const RESULT_LIMIT = 3;

/**
 * The honest five-state machine the dock renders. There is deliberately no
 * "sample" state — an empty result is `empty`, never faked into cards.
 */
export type LandingSearchStatus = 'idle' | 'loading' | 'results' | 'empty' | 'error';

export type LandingSearchState = {
  status: LandingSearchStatus;
  results: TenderResult[];
};

export type LandingSearchOptions = {
  /**
   * Fired once each time a debounced search *resolves* (results or empty) —
   * the analytics hook. Not called on error (there is no result count) and
   * not for superseded responses. Read through a ref so a changing callback
   * identity never re-triggers the debounce.
   */
  onResolved?: (info: { queryLength: number; resultCount: number }) => void;
  /** Overridable only for tests; production always uses {@link DEBOUNCE_MS}. */
  debounceMs?: number;
};

/**
 * Lean, landing-hero search over `tenderClient.searchTenders`: debounced,
 * min-length gated, capped at three rows, no pagination. Anonymous-safe (the
 * transport sends no auth for anon and the server clamps the limit to the anon
 * tier). Keeps the tender-feed hook's request-id race guard so a slow, superseded
 * response can never clobber a newer one — the classic type-fast type-bug.
 */
export function useLandingSearch(
  query: string,
  options: LandingSearchOptions = {},
): LandingSearchState {
  const { onResolved, debounceMs = DEBOUNCE_MS } = options;
  const trimmed = query.trim();

  const [state, setState] = useState<LandingSearchState>({ status: 'idle', results: [] });

  // Monotonic request id: every effect run claims the next id up front, so any
  // still-pending promise from a prior run fails the `=== requestId` check and
  // drops silently — no clobber, no state from a query the user has moved past.
  const requestIdRef = useRef(0);

  // Latest-callback ref, so `onResolved` isn't a debounce dependency.
  const onResolvedRef = useRef(onResolved);
  useEffect(() => {
    onResolvedRef.current = onResolved;
  });

  useEffect(() => {
    const requestId = ++requestIdRef.current;

    if (trimmed.length < MIN_QUERY_LENGTH) {
      setState({ status: 'idle', results: [] });
      return;
    }

    const timer = setTimeout(() => {
      // Enter loading only after the debounce elapses, so a fast typist never
      // sees a spinner flash on every keystroke.
      setState((prev) => ({ status: 'loading', results: prev.results }));

      tenderClient
        .searchTenders({ query: trimmed, limit: RESULT_LIMIT, offset: 0 })
        .then((res) => {
          if (requestIdRef.current !== requestId) return;
          const results = res.results.slice(0, RESULT_LIMIT);
          setState({ status: results.length > 0 ? 'results' : 'empty', results });
          onResolvedRef.current?.({ queryLength: trimmed.length, resultCount: results.length });
        })
        .catch(() => {
          if (requestIdRef.current !== requestId) return;
          setState({ status: 'error', results: [] });
        });
    }, debounceMs);

    return () => clearTimeout(timer);
  }, [trimmed, debounceMs]);

  return state;
}
