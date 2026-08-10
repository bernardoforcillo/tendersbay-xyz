import { useCallback } from 'react';
import type { AssessmentView } from '~/features/company/constants';
import { useAsync } from '~/hooks';
import { companyClient } from '~/lib/api/client';

/**
 * One tender × dossier eligibility evaluation.
 *
 * Component-owned via `useAsync`, deliberately NOT a Zustand slice. The backend
 * computes an assessment fresh on every read and never persists it, because a
 * stored verdict outlives the dossier it was computed against. A client-side
 * cache would resurrect exactly that: a stale, liability-bearing "no-go" shown
 * against a dossier the user has since corrected. The one thing that *is* meant
 * to survive is the decision the human took, and that is what `bid.DecisionRecord`
 * persists server-side.
 *
 * `refetch` is the payoff loop for just-in-time capture: answering a question in
 * a gap row re-runs the check in front of the user, and the pill above their
 * cursor changes.
 */
export function useEligibility(workspaceId: string, tenderId: string, lotRef = '') {
  const fn = useCallback((): Promise<AssessmentView | null> => {
    // A dangling bid has no tender to check against, and the scheda gara mounts
    // before the bid it belongs to has loaded.
    if (!tenderId) return Promise.resolve(null);
    return companyClient
      .checkEligibility({ workspaceId, tenderId, lotRef })
      .then((r) => r.assessment ?? null);
  }, [workspaceId, tenderId, lotRef]);
  return useAsync(fn);
}
