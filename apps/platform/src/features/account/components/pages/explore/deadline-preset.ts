/** Explore "deadline" filter presets, in days from now. */
export const DEADLINE_PRESETS = [7, 30, 90] as const;
export type DeadlinePreset = (typeof DEADLINE_PRESETS)[number];

const ONE_DAY_MS = 86_400_000;

/**
 * Turns a deadline preset (days) into an RFC3339 `{ from, to }` window for
 * `TenderFilters.deadlineFrom/deadlineTo`: from `now` to `now + days`. Returns
 * null for "any time" (no preset) — i.e. no deadline constraint.
 */
export function deadlineRange(
  preset: DeadlinePreset | null,
  now: Date,
): { from: string; to: string } | null {
  if (preset == null) return null;
  return {
    from: now.toISOString(),
    to: new Date(now.getTime() + preset * ONE_DAY_MS).toISOString(),
  };
}
