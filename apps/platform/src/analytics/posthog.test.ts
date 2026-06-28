import { describe, expect, it } from 'vitest';
import { getAnalytics, initAnalytics } from './posthog';

describe('initAnalytics', () => {
  it('returns null and stays uninitialized when no key is configured', () => {
    // VITE_POSTHOG_KEY is unset in the test env.
    expect(initAnalytics()).toBeNull();
    expect(getAnalytics()).toBeNull();
  });
});
