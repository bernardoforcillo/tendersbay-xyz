import type { TenderResult } from '@tendersbay/proto/tender/v1/tender_pb';
import { useEffect, useState } from 'react';
import { tenderClient } from '~/lib/api/client';

/**
 * The top open tenders, fetched once on mount to seed the "Consigliati per te"
 * block on Oggi. This is a recommendation garnish, not a primary flow: any
 * failure degrades to an empty list (no throw, no error surface) so the page
 * simply falls back to the Explore teaser instead of stranding the user on a
 * spinner or an error. A filters-only search (empty query, status=open) leans
 * on the backend to order and cap the results.
 */
export function useRecommendedTenders(): {
  tenders: TenderResult[];
  loading: boolean;
  error: string | null;
} {
  const [tenders, setTenders] = useState<TenderResult[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    setLoading(true);
    setError(null);
    tenderClient
      .searchTenders({ query: '', filters: { status: 'open' }, limit: 4 })
      .then((res) => {
        if (active) setTenders(res.results);
      })
      .catch((e: unknown) => {
        if (!active) return;
        setError(e instanceof Error ? e.message : 'Something went wrong');
        setTenders([]);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, []);

  return { tenders, loading, error };
}
