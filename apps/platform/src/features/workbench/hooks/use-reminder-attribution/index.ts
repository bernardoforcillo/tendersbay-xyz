import { useEffect, useRef } from 'react';
import type { ReminderBucket } from '~/analytics';
import { REMINDER_BUCKETS, useCaptureEvent } from '~/analytics';

/**
 * Record that a reader arrived here from a deadline reminder, then clean the
 * marker out of the URL.
 *
 * This is the click half of the only funnel this product has that starts
 * OUTSIDE it. The digest pass counts what it sends server-side; without this
 * event those counts have no counterpart, and a reminder that pulled someone
 * back is indistinguishable from one deleted unread — which is exactly the
 * number that decides whether reminders earn their place.
 *
 * WHY `window.location.search` rather than the router's `useSearch`: this is a
 * one-shot read of a marker the backend put in a mailed URL, not reactive
 * state, and the route declares no `validateSearch` for it. Going through the
 * router would make the event depend on search-param plumbing that exists for
 * other reasons and could legitimately change.
 *
 * WHY the params are stripped: a reload, a bookmark or a URL pasted to a
 * colleague would otherwise each attribute another click to a reminder that was
 * opened once. `replaceState` is used so the cleanup does not add a history
 * entry the back button has to walk through.
 */
export function useReminderAttribution(bidId: string | undefined): void {
  const capture = useCaptureEvent();
  const done = useRef(false);

  useEffect(() => {
    if (done.current || !bidId) return;

    const params = new URLSearchParams(window.location.search);
    if (params.get('src') !== 'reminder') return;
    done.current = true;

    // The bucket is validated against the closed set rather than trusted: it
    // arrives from a URL anyone can edit, and an unrecognised value must become
    // '' (the allowEmpty case) instead of opening a fifth cohort. The event
    // still fires — a link that lost its bucket is still a reminder that
    // worked, and dropping it would undercount the thing being measured.
    const raw = params.get('b') ?? '';
    const bucket: ReminderBucket | '' = (REMINDER_BUCKETS as readonly string[]).includes(raw)
      ? (raw as ReminderBucket)
      : '';

    capture('reminder_link_opened', { location: 'bid_detail', bucket });

    params.delete('src');
    params.delete('b');
    const query = params.toString();
    window.history.replaceState(
      null,
      '',
      window.location.pathname + (query ? `?${query}` : '') + window.location.hash,
    );
  }, [bidId, capture]);
}
