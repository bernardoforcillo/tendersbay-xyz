const ONE_DAY_MS = 86_400_000;
const URGENT_MAX_DAYS = 7;
const DEADLINE_MAX_DAYS = 14;

export type DeadlineTone = 'urgent' | 'deadline' | 'neutral';

/**
 * Classifies a tender's RFC3339 deadline relative to `now` into a UI tone
 * (urgent/deadline/neutral) plus the day count, so the card and its badge
 * can share one source of truth. Returns null for an empty or unparseable
 * deadline (many tenders have no deadline set).
 */
export function deadlineInfo(
  deadline: string,
  now: Date,
): { tone: DeadlineTone; days: number } | null {
  if (!deadline) return null;
  const target = new Date(deadline);
  if (Number.isNaN(target.getTime())) return null;

  const days = Math.ceil((target.getTime() - now.getTime()) / ONE_DAY_MS);
  const tone: DeadlineTone =
    days <= URGENT_MAX_DAYS ? 'urgent' : days <= DEADLINE_MAX_DAYS ? 'deadline' : 'neutral';
  return { tone, days };
}
