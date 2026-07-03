import posthog, { type PostHog } from 'posthog-js';

let client: PostHog | null = null;

/**
 * Initialize PostHog once, opted out of capturing until the user consents.
 * Returns null (a no-op) when VITE_POSTHOG_KEY is not configured.
 */
export function initAnalytics(): PostHog | null {
  if (client) {
    return client;
  }
  // Prefer runtime config injected by the Go server (window.__ENV__); fall back
  // to the build-time value (Vite/.env) in dev.
  const key = window.__ENV__?.POSTHOG_KEY ?? import.meta.env.VITE_POSTHOG_KEY;
  if (!key) {
    return null;
  }
  posthog.init(key, {
    api_host:
      window.__ENV__?.POSTHOG_HOST ??
      import.meta.env.VITE_POSTHOG_HOST ??
      'https://eu.i.posthog.com',
    opt_out_capturing_by_default: true,
    autocapture: true,
    capture_pageview: 'history_change',
    capture_exceptions: true,
    disable_session_recording: false,
    session_recording: { maskAllInputs: true },
    persistence: 'localStorage+cookie',
  });
  client = posthog;
  return client;
}

export function getAnalytics(): PostHog | null {
  return client;
}
