import { useCallback, useState } from 'react';
import type { DocumentAvailabilityView, PassageView } from '~/features/tenders/constants';
import { tenderClient } from '~/lib/api/client';

export type TenderPassagesResult = {
  availability: DocumentAvailabilityView | null;
  passages: PassageView[];
};

/**
 * Reads passages of a tender's documents around a question.
 *
 * Deliberately NOT built on `useAsync`: every other server read on this surface
 * fires on mount, and this one must not. A quote's surrounding passage is a
 * metered retrieval the reader asks for by expanding a disclosure — firing it
 * for every citation on a scheda gara would spend the capability on passages
 * nobody opened. Hence a manual `load(question)` the caller invokes.
 *
 * `availability` always travels with the result, never passages alone: an empty
 * list without a cause conflates "no passage matched" with "we hold nothing but
 * the notice". The bounds (max passages, max runes) are enforced in the backend's
 * core, so `limit` here is a hint the server may only narrow.
 */
export function useTenderPassages(tenderId: string) {
  const [data, setData] = useState<TenderPassagesResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(
    async (question: string, limit = 3) => {
      setLoading(true);
      setError(null);
      try {
        const res = await tenderClient.getTenderPassages({ id: tenderId, question, limit });
        setData({
          availability: res.availability ?? null,
          passages: res.passages ?? [],
        });
      } catch (e: unknown) {
        setError(e instanceof Error ? e.message : 'Something went wrong');
        setData(null);
      } finally {
        setLoading(false);
      }
    },
    [tenderId],
  );

  return { data, loading, error, load };
}
