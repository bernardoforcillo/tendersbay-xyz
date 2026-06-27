import { PostHogProvider } from 'posthog-js/react';
import type { ReactNode } from 'react';
import { getAnalytics } from './posthog';

/**
 * Wraps children in the PostHog React provider only when a client exists, so
 * the app renders unchanged when analytics is disabled (no VITE_POSTHOG_KEY).
 */
export function AnalyticsProvider({ children }: { children: ReactNode }) {
  const client = getAnalytics();
  if (!client) {
    return <>{children}</>;
  }
  return <PostHogProvider client={client}>{children}</PostHogProvider>;
}
