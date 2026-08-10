import { useCallback } from 'react';
import type { DocumentAvailabilityView } from '~/features/tenders/constants';
import { useAsync } from '~/hooks';
import { tenderClient } from '~/lib/api/client';

/**
 * What we hold of a tender's documents, without retrieving any passage.
 *
 * It is the same RPC `useTenderPassages` uses, called with an EMPTY question —
 * the server short-circuits retrieval and answers with availability alone. That
 * is deliberate rather than a separate endpoint: availability and passages are
 * one fact on the wire, and splitting them into two RPCs would eventually let a
 * caller obtain passages without the cause that qualifies them, which is the
 * exact conflation the pairing exists to prevent.
 *
 * An empty `tenderId` resolves to null rather than calling: the scheda gara
 * mounts before the bid it belongs to has loaded.
 */
export function useTenderAvailability(tenderId: string) {
  const fn = useCallback((): Promise<DocumentAvailabilityView | null> => {
    if (!tenderId) return Promise.resolve(null);
    return tenderClient
      .getTenderPassages({ id: tenderId, question: '', limit: 0 })
      .then((r) => r.availability ?? null);
  }, [tenderId]);
  return useAsync(fn);
}
