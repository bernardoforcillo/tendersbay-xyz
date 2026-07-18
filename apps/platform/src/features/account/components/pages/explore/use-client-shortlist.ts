import type { RecommendedTenderResult } from '@tendersbay/proto/tender/v1/tender_pb';
import { useEffect, useState } from 'react';
import { tenderClient } from '~/lib/api/client';

const SHORTLIST_LIMIT = 3;

/**
 * The per-client best-fit shortlist for Explore's default state — a thin sibling of
 * use-tender-search.ts, but reading RecommendTendersForClient (deterministic, unmetered,
 * membership-checked) instead of the free-text SearchTenders. Re-fetches whenever
 * workspaceId changes (the advisor switching clients); a null workspaceId (no client
 * selected) short-circuits without a network call.
 */
export function useClientShortlist(workspaceId: string | null): {
  results: RecommendedTenderResult[];
  needsProfile: boolean;
  loading: boolean;
  error: string | null;
} {
  const [results, setResults] = useState<RecommendedTenderResult[]>([]);
  const [needsProfile, setNeedsProfile] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!workspaceId) {
      setResults([]);
      setNeedsProfile(false);
      setLoading(false);
      setError(null);
      return;
    }
    let active = true;
    setLoading(true);
    setError(null);
    tenderClient
      .recommendTendersForClient({ workspaceId, limit: SHORTLIST_LIMIT })
      .then((res) => {
        if (!active) return;
        setResults(res.results);
        setNeedsProfile(res.needsProfile);
      })
      .catch((e: unknown) => {
        if (!active) return;
        setError(e instanceof Error ? e.message : 'Something went wrong');
        setResults([]);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [workspaceId]);

  return { results, needsProfile, loading, error };
}
