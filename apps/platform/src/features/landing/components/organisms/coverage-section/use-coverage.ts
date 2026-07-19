import { useEffect, useState } from 'react';
import {
  EU_COUNTRIES,
  type EuCountry,
} from '~/features/landing/components/atoms/country-flag/flags';
import { tenderClient } from '~/lib/api/client';

const EU = new Set<string>(EU_COUNTRIES);

/**
 * Live coverage from the backend: the set of EU countries we currently hold
 * tenders for (DISTINCT country over ingested_tenders). Fetched once on mount,
 * anon-safe. Any failure degrades to an empty Set so the marquee falls back to
 * the all-coming-soon teaser instead of stranding on a spinner — coverage is a
 * hint, never a blocker. Non-EU codes are filtered out so the marquee never
 * lights a flag it has no component for.
 */
export function useCoverage(): ReadonlySet<EuCountry> {
  const [available, setAvailable] = useState<ReadonlySet<EuCountry>>(() => new Set());

  useEffect(() => {
    let cancelled = false;
    tenderClient
      .getCoverage({})
      .then((res) => {
        if (cancelled) return;
        const codes = res.countries.filter((c): c is EuCountry => EU.has(c));
        setAvailable(new Set(codes));
      })
      .catch(() => {
        /* teaser fallback: leave the set empty */
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return available;
}
