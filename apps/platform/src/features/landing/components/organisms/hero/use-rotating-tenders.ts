import { useEffect, useState } from 'react';
import type { Tender } from '~/features/landing/components/atoms';

function prefersReducedMotion(): boolean {
  return window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false;
}

export function useRotatingTenders(
  tenders: Tender[],
  intervalMs = 3200,
): { tender: Tender; index: number } {
  const [index, setIndex] = useState(0);

  useEffect(() => {
    if (tenders.length <= 1 || prefersReducedMotion()) {
      return;
    }
    const id = setInterval(() => {
      setIndex((current) => (current + 1) % tenders.length);
    }, intervalMs);
    return () => clearInterval(id);
  }, [tenders.length, intervalMs]);

  const safeIndex = index % tenders.length;
  return { tender: tenders[safeIndex] as Tender, index: safeIndex };
}
