import type { TenderDetail } from '@tendersbay/proto/tender/v1/tender_pb';
import { useCallback } from 'react';
import { useAsync } from '~/hooks';
import { tenderClient } from '~/lib/api/client';

/**
 * One tender's detail, for surfaces that already know the id.
 *
 * Distinct from `loadTenderDetail`, which is the public page's route loader and
 * additionally fetches the related list and translates a not-found into a route
 * state. The scheda gara wants neither: it needs the award grid and the
 * documents URL, and a failure there must degrade the criteria block, not the
 * page.
 *
 * An empty `tenderId` resolves to null — a dangling bid has no tender to read.
 */
export function useTenderDetail(tenderId: string) {
  const fn = useCallback((): Promise<TenderDetail | null> => {
    if (!tenderId) return Promise.resolve(null);
    return tenderClient.getTender({ id: tenderId }).then((r) => r.tender ?? null);
  }, [tenderId]);
  return useAsync(fn);
}
