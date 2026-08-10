import { usePostHog } from 'posthog-js/react';
import { useCallback } from 'react';
import {
  type AnalyticsEventName,
  type AnalyticsEventProps,
  captureAnalyticsEvent,
} from '../../events';

/**
 * Returns a typed `capture` bound to the mounted PostHog client.
 *
 * The house idiom is `usePostHog()` then `posthog?.capture('name', { … })`, and
 * this changes exactly one thing about it: the event name and its properties are
 * checked against `~/analytics/events`, so a typo in an event name, a missing
 * `location`, or a stray free-text property is a compile error instead of a
 * silently-diverging funnel. The optional chaining, the absence of a consent
 * guard, and the absence of a `locale` prop are unchanged — `usePostHog()`
 * returns null when analytics is disabled (tests, no `VITE_POSTHOG_KEY`),
 * PostHog drops events captured before consent, and `useLocaleProperty` already
 * registers `locale` as a super-property.
 */
export function useCaptureEvent(): <K extends AnalyticsEventName>(
  name: K,
  props: AnalyticsEventProps[K],
) => void {
  const posthog = usePostHog();
  return useCallback(
    <K extends AnalyticsEventName>(name: K, props: AnalyticsEventProps[K]) => {
      captureAnalyticsEvent(posthog, name, props);
    },
    [posthog],
  );
}
