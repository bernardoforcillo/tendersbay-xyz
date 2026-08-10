import { describe, expect, it, vi } from 'vitest';
import {
  ANALYTICS_EVENT_NAMES,
  type AnalyticsEventName,
  analyticsEventProps,
  captureAnalyticsEvent,
  EVENT_SPECS,
  INVALID_VALUE,
} from './index';

describe('the event catalogue', () => {
  it('gives every event a location', () => {
    for (const name of ANALYTICS_EVENT_NAMES) {
      expect(Object.keys(EVENT_SPECS[name]), name).toContain('location');
    }
  });

  it('names every event snake_case object_verb in the past tense', () => {
    const pastTense =
      /^[a-z0-9]+(_[a-z0-9]+)*_(shown|opened|recommended|overridden|recorded|answered|skipped|confirmed|viewed)$/;
    for (const name of ANALYTICS_EVENT_NAMES) {
      expect(name, name).toMatch(pastTense);
    }
  });

  it('never carries a locale prop — it is already a super-property', () => {
    for (const name of ANALYTICS_EVENT_NAMES) {
      expect(Object.keys(EVENT_SPECS[name]), name).not.toContain('locale');
    }
  });

  /**
   * The Phase 1a lesson, made mechanical: "criteria are published" overstates
   * "the scoring grid is usable" by ~2x. The only way to guarantee nobody
   * reports the misleading number is to keep an existence boolean about criteria
   * out of the catalogue entirely.
   */
  it('exposes no boolean about criteria anywhere', () => {
    for (const name of ANALYTICS_EVENT_NAMES) {
      const spec = EVENT_SPECS[name] as Record<string, { kind: string }>;
      for (const [key, prop] of Object.entries(spec)) {
        if (key.includes('criteria')) {
          expect(prop.kind, `${name}.${key}`).toBe('count');
        }
      }
    }
  });

  it('never lets a criteria count be reported without its weighted count', () => {
    const spec = Object.keys(EVENT_SPECS.tender_criteria_viewed);
    expect(spec).toContain('criteria_count');
    expect(spec).toContain('weighted_criteria_count');
  });
});

describe('analyticsEventProps', () => {
  it('emits only the properties the spec declares', () => {
    const props = analyticsEventProps('requirement_confirmed', {
      location: 'bid_detail',
      kind: 'soa',
      source: 'document_excerpt',
      had_excerpt: true,
      had_citation: false,
    });
    expect(Object.keys(props).sort()).toEqual([
      'had_citation',
      'had_excerpt',
      'kind',
      'location',
      'source',
    ]);
  });

  /**
   * The privacy guarantee, tested at its weakest point: a caller that has cast
   * its way past the types. Nothing outside the spec is copied, so no company
   * name, requirement text or document URL can ride along.
   */
  it('drops undeclared properties even when a caller casts past the types', () => {
    const smuggled = {
      location: 'bid_detail',
      kind: 'soa',
      source: 'document_excerpt',
      had_excerpt: true,
      had_citation: true,
      requirement_text: 'Attestazione SOA OG1 classifica III',
      document_url: 'https://buyer.example/disciplinare.pdf',
      company_name: 'Acme Costruzioni S.r.l.',
      vat_number: 'IT01234567890',
    } as unknown as Parameters<typeof analyticsEventProps<'requirement_confirmed'>>[1];

    const props = analyticsEventProps('requirement_confirmed', smuggled);

    expect(props).not.toHaveProperty('requirement_text');
    expect(props).not.toHaveProperty('document_url');
    expect(props).not.toHaveProperty('company_name');
    expect(props).not.toHaveProperty('vat_number');
    expect(Object.values(props)).not.toContain('Acme Costruzioni S.r.l.');
  });

  it('replaces an out-of-vocabulary string rather than forwarding it', () => {
    const props = analyticsEventProps('company_dossier_field_answered', {
      location: 'company_settings',
      field_category: 'Acme Costruzioni S.r.l.' as never,
      prompted_by: 'settings',
    });
    expect(props.field_category).toBe(INVALID_VALUE);
    // The event still fires with the rest intact: scrubbing a value must not
    // cost the observation, or every rate computed from it drifts.
    expect(props.location).toBe('company_settings');
    expect(props.prompted_by).toBe('settings');
  });

  it('admits the empty recommendation but not an arbitrary empty enum', () => {
    // "" means no eligibility check existed — a different fact from
    // `insufficient_data`, and one the override rate needs to keep separate.
    expect(
      analyticsEventProps('bid_decision_recorded', {
        location: 'bid_detail',
        decision: 'go',
        recommendation: '',
        overridden: false,
      }).recommendation,
    ).toBe('');

    expect(
      analyticsEventProps('bid_decision_recorded', {
        location: 'bid_detail',
        decision: '' as never,
        recommendation: 'no_go',
        overridden: true,
      }).decision,
    ).toBe(INVALID_VALUE);
  });

  it('keeps "absent on the wire" apart from "a value we do not recognise"', () => {
    // A backend that has not shipped coverage yet is a rollout; a backend
    // sending a coverage code this build never heard of is a drift alarm. One
    // bucket for both would fire the alarm on every deploy.
    const absent = analyticsEventProps('eligibility_gap_shown', {
      location: 'scheda_gara',
      gap_count: 0,
      blocking: false,
      unknown_count: 0,
      coverage: '',
      reason: '',
    });
    expect(absent.coverage).toBe('');
    expect(absent.reason).toBe('');

    const drifted = analyticsEventProps('eligibility_gap_shown', {
      location: 'scheda_gara',
      gap_count: 0,
      blocking: false,
      unknown_count: 0,
      coverage: 'partially_read' as never,
      reason: 'fetch_timed_out' as never,
    });
    expect(drifted.coverage).toBe(INVALID_VALUE);
    expect(drifted.reason).toBe(INVALID_VALUE);
  });

  it('scrubs an empty code-supplied enum — nothing on the wire produces one', () => {
    expect(
      analyticsEventProps('repair_chat_opened', {
        location: 'bid_detail',
        seeded_from: '' as never,
      }).seeded_from,
    ).toBe(INVALID_VALUE);
    expect(
      analyticsEventProps('citation_opened', {
        location: '' as never,
        surface: 'dossier',
        has_section: false,
        has_page: false,
      }).location,
    ).toBe(INVALID_VALUE);
  });

  it('keeps grid_usable tri-state and never coerces absent to false', () => {
    const base = {
      location: 'tender_detail',
      criteria_count: 4,
      weighted_criteria_count: 0,
      lot_count: 1,
    } as const;
    expect(
      analyticsEventProps('tender_criteria_viewed', { ...base, grid_usable: null }).grid_usable,
    ).toBeNull();
    expect(
      analyticsEventProps('tender_criteria_viewed', {
        ...base,
        grid_usable: undefined as never,
      }).grid_usable,
    ).toBeNull();
    expect(
      analyticsEventProps('tender_criteria_viewed', { ...base, grid_usable: false }).grid_usable,
    ).toBe(false);
    expect(
      analyticsEventProps('tender_criteria_viewed', { ...base, grid_usable: true }).grid_usable,
    ).toBe(true);
  });

  it('normalises counts to non-negative integers', () => {
    const props = analyticsEventProps('eligibility_gap_shown', {
      location: 'scheda_gara',
      gap_count: 3.7,
      blocking: true,
      unknown_count: -2,
      coverage: 'notice_only',
      reason: 'body_not_retrieved',
    });
    expect(props.gap_count).toBe(3);
    expect(props.unknown_count).toBe(0);
  });

  it('records an override with both sides of the disagreement', () => {
    const props = analyticsEventProps('go_no_go_overridden', {
      location: 'bid_detail',
      from: 'no_go',
      to: 'go',
      blocking_gap_count: 2,
    });
    expect(props).toEqual({
      location: 'bid_detail',
      from: 'no_go',
      to: 'go',
      blocking_gap_count: 2,
    });
  });
});

describe('captureAnalyticsEvent', () => {
  it('captures unconditionally through the sink', () => {
    const capture = vi.fn();
    captureAnalyticsEvent({ capture } as never, 'repair_chat_opened', {
      location: 'bid_detail',
      seeded_from: 'gap',
    });
    expect(capture).toHaveBeenCalledWith('repair_chat_opened', {
      location: 'bid_detail',
      seeded_from: 'gap',
    });
  });

  it('is a no-op when analytics is disabled', () => {
    expect(() =>
      captureAnalyticsEvent(null, 'citation_opened', {
        location: 'bid_detail',
        surface: 'dossier',
        has_section: true,
        has_page: false,
      }),
    ).not.toThrow();
  });

  it('scrubs an unknown location instead of shipping it', () => {
    const capture = vi.fn();
    const name: AnalyticsEventName = 'document_coverage_shown';
    captureAnalyticsEvent({ capture } as never, name, {
      location: 'somewhere_else' as never,
      coverage: 'full',
      reason: '',
      known_document_links: 3,
      extracted_documents: 3,
    });
    expect(capture).toHaveBeenCalledWith(name, {
      location: INVALID_VALUE,
      coverage: 'full',
      reason: '',
      known_document_links: 3,
      extracted_documents: 3,
    });
  });
});
