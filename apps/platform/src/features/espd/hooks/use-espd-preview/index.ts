import { useCallback } from 'react';
import { useAsync } from '~/hooks';
import { espdClient } from '~/lib/api/client';

/**
 * The composed DGUE for one bid: which fields are filled, which are open, and
 * whether it can be exported.
 *
 * A read, not a store. The preview is computed fresh on every call — the server
 * never persists it — so caching it in Zustand would put a stale readiness
 * count on screen next to a dossier the user just changed, which is the one
 * thing this surface must never do. The refetch after a capture is what makes
 * an answer visibly move the counter, exactly as the eligibility check does.
 */
export function useEspdPreview(workbenchId: string, bidId: string) {
  const fn = useCallback(
    () => espdClient.getResponsePreview({ workbenchId, bidId }),
    [workbenchId, bidId],
  );
  return useAsync(fn);
}
