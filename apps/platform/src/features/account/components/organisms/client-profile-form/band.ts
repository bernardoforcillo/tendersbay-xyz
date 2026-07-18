/**
 * Buckets a small count into a consent-safe PostHog prop — never send the
 * raw count for something a user configured (per add-posthog-metrics: bands,
 * not exact figures).
 */
export function band(n: number): string {
  if (n <= 0) return '0';
  if (n === 1) return '1';
  if (n <= 3) return '2-3';
  return '4+';
}
