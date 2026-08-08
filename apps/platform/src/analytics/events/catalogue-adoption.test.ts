import { describe, expect, it } from 'vitest';
import { ANALYTICS_EVENT_NAMES } from './index';

/**
 * Every catalogued event must be fired THROUGH the catalogue.
 *
 * WHY this is a test and not a review note: the three guarantees
 * `~/analytics/events` states in its header — no free text can be passed, an
 * undeclared key is never copied, a value outside its vocabulary becomes
 * `invalid` rather than being forwarded — are properties of
 * `analyticsEventProps`, not of the event name. A raw
 * `posthog?.capture('go_no_go_recommended', { … })` produces an event PostHog
 * accepts and a funnel that looks right, while holding none of them. Nothing
 * about it is visibly wrong at the call site, which is precisely why it needs a
 * mechanical check rather than a convention.
 *
 * It also catches the drift that is invisible any other way: a `location` typed
 * `string` instead of `AnalyticsLocation`. `company-dossier-form` shipped
 * `'settings_company'` against a catalogue that names the surface
 * `'company_settings'`, which would have put every up-front dossier answer in the
 * `invalid` bucket and left the PRD's central hypothesis with one measurable arm.
 *
 * Uncatalogued events (`bando_tracked`, `bid_stage_changed`, …) are deliberately
 * out of scope — they predate this catalogue and are not claimed by it. Adding
 * one HERE is what brings it under these rules.
 */
const sources = import.meta.glob('../../features/**/*.{ts,tsx}', {
  eager: true,
  query: '?raw',
  import: 'default',
}) as Record<string, string>;

describe('catalogue adoption', () => {
  const production = Object.entries(sources).filter(([path]) => !path.includes('.test.'));

  it('reads a non-trivial number of feature sources', () => {
    // Guards the glob itself: a pattern that silently matched nothing would make
    // every assertion below vacuously true.
    expect(production.length).toBeGreaterThan(50);
  });

  it.each(ANALYTICS_EVENT_NAMES)('fires %s only through the typed catalogue', (name) => {
    const offenders = production
      .filter(([, source]) => rawFires(source, name))
      .map(([path]) => path);
    expect(offenders, `${name} is captured raw in these files`).toEqual([]);
  });
});

/** True when `name` is fired via a bare posthog client rather than the catalogue. */
function rawFires(source: string, name: string): boolean {
  return new RegExp(String.raw`posthog\s*\??\.\s*capture\(\s*'${name}'`).test(source);
}
