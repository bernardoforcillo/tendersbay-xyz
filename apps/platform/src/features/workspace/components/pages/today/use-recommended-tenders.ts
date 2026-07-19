import { useEffect, useState } from 'react';
import { tenderClient } from '~/lib/api/client';

/**
 * The client's best-fit tender COUNT, for the thin "N best-fit ready" nudge on Today — a
 * habit-loop entry point that deep-links into Explore, not the interaction home (see the
 * design spec §2). Any failure degrades to a zero count (no throw, no error surface) so the
 * page simply falls back to the Explore teaser instead of stranding the user on a spinner.
 */
export function useRecommendedTenders(workspaceId: string): {
  count: number;
  needsProfile: boolean;
  loading: boolean;
  error: string | null;
} {
  const [count, setCount] = useState(0);
  const [needsProfile, setNeedsProfile] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    setLoading(true);
    setError(null);
    tenderClient
      .recommendTendersForClient({ workspaceId, limit: 4 })
      .then((res) => {
        if (!active) return;
        setCount(res.results.length);
        setNeedsProfile(res.needsProfile);
      })
      .catch((e: unknown) => {
        if (!active) return;
        setError(e instanceof Error ? e.message : 'Something went wrong');
        setCount(0);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [workspaceId]);

  return { count, needsProfile, loading, error };
}
