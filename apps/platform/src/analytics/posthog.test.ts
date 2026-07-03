import { afterEach, describe, expect, it, vi } from 'vitest';
import { getAnalytics, initAnalytics } from './posthog';

describe('initAnalytics', () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it('returns null and stays uninitialized when no key is configured', () => {
    // Force the unconfigured branch deterministically, regardless of any local
    // .env (which Vitest loads) or runtime window.__ENV__ injection.
    vi.stubEnv('VITE_POSTHOG_KEY', '');
    expect(initAnalytics()).toBeNull();
    expect(getAnalytics()).toBeNull();
  });
});
