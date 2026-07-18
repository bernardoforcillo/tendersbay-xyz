export const LANDING_CARRY_OVER_KEY = 'tb.landing.lastSearch';

export type LandingCarryOver = {
  query: string;
  filters: Record<string, unknown>;
};

/**
 * Persists the visitor's last landing-page search so the first-run client-profile
 * capture (shown once a workspace exists) can pre-fill Notes with it. Anonymous,
 * pre-auth state — sessionStorage only, never sent to the server. Storage errors
 * (private mode, quota) are swallowed: the carry-over is a nice-to-have, never a
 * blocker for the search itself.
 */
export function writeLandingCarryOver(data: LandingCarryOver): void {
  try {
    sessionStorage.setItem(LANDING_CARRY_OVER_KEY, JSON.stringify(data));
  } catch {
    /* storage unavailable — carry-over is best-effort */
  }
}

/**
 * Reads the carry-over once and clears it, so a stale search never resurfaces
 * against a later, unrelated workspace. Returns null for missing, malformed, or
 * empty-query entries.
 */
export function readAndClearLandingCarryOver(): LandingCarryOver | null {
  try {
    const raw = sessionStorage.getItem(LANDING_CARRY_OVER_KEY);
    if (!raw) return null;
    sessionStorage.removeItem(LANDING_CARRY_OVER_KEY);
    const parsed = JSON.parse(raw) as Partial<LandingCarryOver>;
    if (typeof parsed.query !== 'string' || parsed.query.trim() === '') return null;
    return { query: parsed.query, filters: parsed.filters ?? {} };
  } catch {
    return null;
  }
}
