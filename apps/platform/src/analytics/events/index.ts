import type { PostHog } from 'posthog-js';
import {
  REQUIREMENT_KINDS,
  REQUIREMENT_SOURCES,
  type RequirementKind,
  type RequirementSource,
  VERDICTS,
  type Verdict,
} from '~/features/company/constants';
import {
  ESPD_FORMATS,
  ESPD_VERSIONS,
  type EspdFormat,
  type EspdVersion,
  GAP_REASONS,
  GAP_SCOPES,
  type GapReason,
  type GapScope,
} from '~/features/espd/constants';
import {
  COVERAGE_REASONS,
  COVERAGE_VALUES,
  type Coverage,
  type CoverageReason,
} from '~/features/tenders/constants';

/**
 * The scheda gara's product-analytics vocabulary, declared once.
 *
 * WHY this exists rather than eleven inline `posthog?.capture('…', { … })` calls
 * scattered across three features: every one of these events reports on a
 * decision a user is about to take on real, commercially sensitive procurement
 * work. The event payloads must carry counts and closed-vocabulary codes and
 * NOTHING else — no requirement text, no quoted passage, no document URL, no
 * buyer, no VAT, no certification identifier. That rule is easy to state and
 * easy to break by accident (a debugging `excerpt` prop that ships, a `title`
 * added "just to see"), so it is enforced here structurally instead of by
 * review:
 *
 *   1. Every event's payload type admits only literal-union strings, numbers and
 *      booleans. There is no `string` in any signature below, so free text
 *      cannot be passed in the first place.
 *   2. `analyticsEventProps` builds the outgoing payload by walking the event's
 *      SPEC, not the caller's object. A key the spec does not declare is not
 *      copied — so even a caller that widens a type with `as` cannot add one.
 *   3. A string value that is not in its declared vocabulary is replaced with
 *      `INVALID_VALUE`, never forwarded. The event still fires (the denominator
 *      stays honest) and a non-zero `invalid` bucket in PostHog is itself the
 *      alarm that the frontend vocabulary has drifted from the Go constants.
 *
 * The vocabularies are IMPORTED from `~/features/{company,tenders}/constants`
 * rather than re-declared here on purpose. Those arrays are already the single
 * mechanical link between a Go constant and its 24 translations; a second copy
 * would drift, and a drifted copy here fails silently — valid values scrubbed to
 * `invalid` — which is worse than a loud break.
 *
 * House idiom this follows (`.claude/memory/landing-analytics.md`): snake_case
 * `object_verb` past tense, a `location` on every event, capture unconditionally
 * (opt-out gating is automatic, `opt_out_capturing_by_default: true`), and never
 * a `locale` prop — `useLocaleProperty` registers it as a super-property.
 */

/** Replaces any string that is not in its declared vocabulary. Never a domain value. */
export const INVALID_VALUE = 'invalid';

/**
 * The surfaces an event can be fired from.
 *
 * Deliberately closed. `location` is the axis every one of these events is read
 * along — a just-in-time answer captured on the scheda gara and one typed into
 * settings are the two arms of the PRD's central hypothesis, and they are only
 * distinguishable if the surface names are a fixed set rather than whatever
 * string a component happened to pass down. Adding a surface means adding it
 * here.
 */
export const ANALYTICS_LOCATIONS = [
  'bid_detail',
  'scheda_gara',
  'tender_detail',
  'company_settings',
  'workbench',
  'explore',
] as const;
export type AnalyticsLocation = (typeof ANALYTICS_LOCATIONS)[number];

/**
 * Where the reader was standing when they opened a citation. A quote expanded
 * from the dossier is a verification act; one expanded in chat is a reading act.
 */
export const CITATION_SURFACES = ['dossier', 'chat'] as const;
export type CitationSurface = (typeof CITATION_SURFACES)[number];

/**
 * What a dossier answer was about. `identity` has no requirement kind — it is
 * never a blocking gap and is only ever typed into settings — so it extends the
 * requirement kinds rather than living in them.
 */
export const DOSSIER_FIELD_CATEGORIES = [...REQUIREMENT_KINDS, 'identity'] as const;
export type DossierFieldCategory = RequirementKind | 'identity';

/**
 * Which arm of the capture hypothesis an answer came from. This single property
 * is what makes just-in-time-versus-up-front measurable at all.
 */
export const CAPTURE_PROMPTERS = ['tender', 'settings'] as const;
export type CapturePrompter = (typeof CAPTURE_PROMPTERS)[number];

/** A recorded human decision. Distinct from a `Verdict`, which is the machine's. */
export const DECISIONS = ['go', 'no_go'] as const;
export type Decision = (typeof DECISIONS)[number];

/** How the repair chat came to be open. */
export const CHAT_SEEDS = ['gap', 'manual'] as const;
export type ChatSeed = (typeof CHAT_SEEDS)[number];

/**
 * The `Reported*` family: a wire-sourced enum, plus `''` for "the field was not
 * there".
 *
 * WHY absent is admitted rather than scrubbed. Every one of these values is read
 * off a proto3 string field, which is `''` when the server did not set it, and
 * `''` is a DIFFERENT fact from a value nobody recognises. Folding both into
 * `INVALID_VALUE` would put "an older backend does not send coverage yet" in the
 * same bucket as "the backend sent a coverage code this build has never heard
 * of" — one is a rollout, the other is a drift alarm, and a bucket that cannot
 * tell them apart raises a false alarm on every deploy.
 *
 * For `Verdict` the empty string carries an explicit domain meaning as well: on
 * `bid_decision_recorded` / `go_no_go_overridden` it means NO eligibility check
 * existed when the decision was recorded — not `insufficient_data`, which means
 * a check ran and the evidence was thin (`core/bid/types.go`). Collapsing those
 * two is what makes an override rate un-interpretable.
 *
 * The split is exactly wire-sourced versus code-supplied: `location`, `surface`,
 * `prompted_by`, `seeded_from`, `to` and `decision` are chosen by this codebase,
 * never read off a message, so an empty one of those is a bug and is scrubbed.
 */
export type ReportedCoverage = Coverage | '';
export type ReportedCoverageReason = CoverageReason | '';
export type ReportedVerdict = Verdict | '';
export type ReportedRequirementKind = RequirementKind | '';
export type ReportedRequirementSource = RequirementSource | '';
/**
 * The ESPD reads. Scope and reason come off the wire and can be empty; a
 * version or a format is chosen by the export sheet's own buttons, so neither
 * has an empty arm — an empty one there would be a bug, and is scrubbed.
 */
export type ReportedGapScope = GapScope | '';
export type ReportedGapReason = GapReason | '';
export type ReportedEspdVersion = EspdVersion;
export type ReportedEspdFormat = EspdFormat;
export type ReportedFieldCategory = DossierFieldCategory | '';

/**
 * The event catalogue. One entry per event; the property types are the contract.
 *
 * Note what is NOT here, deliberately:
 *
 *   - No `confidence_band` on `go_no_go_recommended`. Nothing on the wire
 *     produces one, and a single scalar would hide the distinction that decides
 *     what to build next: `insufficient_data` because we read nothing, versus
 *     because the user has not answered a question. `caveat_count` plus
 *     `eligibility_gap_shown{coverage, reason}` carry that losslessly.
 *   - No `has_criteria`-style existence boolean anywhere. Phase 1a measured that
 *     "criteria are published" overstates "the scoring grid is usable" by ~2x,
 *     because roughly 64% of Italian notices name criteria and publish no
 *     weights. The only way to guarantee nobody reports the misleading number is
 *     to not make it available: `tender_criteria_viewed` carries
 *     `weighted_criteria_count` next to `criteria_count`, always, and a test
 *     below asserts no boolean about criteria ever enters this catalogue.
 */
export type AnalyticsEventProps = {
  /** How much of the tender's document text we actually hold, and why. */
  document_coverage_shown: {
    location: AnalyticsLocation;
    coverage: ReportedCoverage;
    reason: ReportedCoverageReason;
    known_document_links: number;
    extracted_documents: number;
  };
  /**
   * A reader expanded the passage behind a quote. `has_section`/`has_page` are
   * the only production read on whether Phase 1a's page/section metadata landed
   * — never the page number itself, never the document URL.
   */
  citation_opened: {
    location: AnalyticsLocation;
    surface: CitationSurface;
    has_section: boolean;
    has_page: boolean;
  };
  /**
   * Coverage travels with the gap counts so the two are joinable: a no-go
   * computed over the notice alone and one computed over the disciplinare are
   * not the same claim, and a gap rate that cannot tell them apart is noise.
   */
  eligibility_gap_shown: {
    location: AnalyticsLocation;
    gap_count: number;
    blocking: boolean;
    unknown_count: number;
    coverage: ReportedCoverage;
    reason: ReportedCoverageReason;
  };
  go_no_go_recommended: {
    location: AnalyticsLocation;
    recommendation: ReportedVerdict;
    caveat_count: number;
    authoritative_requirement_count: number;
    requirement_count: number;
  };
  /**
   * The single most important event in the set: the only direct read on whether
   * users trust the recommendation. `from` and `to` are both mandatory — an
   * override with only one side recorded says a user disagreed but not with
   * what, which answers nothing.
   */
  go_no_go_overridden: {
    location: AnalyticsLocation;
    from: ReportedVerdict;
    to: Decision;
    blocking_gap_count: number;
  };
  bid_decision_recorded: {
    location: AnalyticsLocation;
    decision: Decision;
    recommendation: ReportedVerdict;
    overridden: boolean;
  };
  company_dossier_field_answered: {
    location: AnalyticsLocation;
    field_category: ReportedFieldCategory;
    prompted_by: CapturePrompter;
  };
  /**
   * The denominator `company_dossier_field_answered` alone does not have.
   * Just-in-time versus up-front capture is an answered/asked ratio; without the
   * skip the hypothesis is untestable.
   */
  company_dossier_question_skipped: {
    location: AnalyticsLocation;
    field_category: ReportedFieldCategory;
  };
  requirement_confirmed: {
    location: AnalyticsLocation;
    kind: ReportedRequirementKind;
    source: ReportedRequirementSource;
    had_excerpt: boolean;
    had_citation: boolean;
  };
  /**
   * `grid_usable` is tri-state on purpose: `null` means the notice was never
   * enriched, which is NOT the same as "the notice names an award method but
   * publishes no weights". A boolean would record ignorance as a finding.
   */
  tender_criteria_viewed: {
    location: AnalyticsLocation;
    criteria_count: number;
    weighted_criteria_count: number;
    grid_usable: boolean | null;
    lot_count: number;
  };
  repair_chat_opened: {
    location: AnalyticsLocation;
    seeded_from: ChatSeed;
  };

  // ── ESPD / DGUE ───────────────────────────────────────────────────────────
  //
  // The readiness numbers travel as COUNTS and never as a percentage: a
  // percentage cannot tell "3 of 4" from "300 of 400", and the whole point of
  // splitting the to-do list is that the two denominators move differently.
  // Nothing here carries a criterion key, a buyer, a reference or a filename —
  // a criterion key is a statement about a company's legal standing, and the
  // fact that a workspace answered "yes" to a conviction ground must not be
  // reconstructable from analytics.

  /**
   * The readiness card was shown. `company_gap_count` and `bid_gap_count` are
   * separate because they answer different product questions: the first decays
   * to zero across a workspace's whole life, the second resets every gara.
   */
  dgue_readiness_viewed: {
    location: AnalyticsLocation;
    ready: boolean;
    ready_field_count: number;
    gap_count: number;
    company_gap_count: number;
    bid_gap_count: number;
    declarations_complete: boolean;
    declarations_confirmed: boolean;
    request_known: boolean;
  };
  /**
   * A gap was closed from the readiness card. `scope` and `reason` are what
   * make this joinable to the card above; the criterion itself is not sent.
   */
  dgue_gap_filled: {
    location: AnalyticsLocation;
    scope: ReportedGapScope;
    reason: ReportedGapReason;
  };
  /**
   * The Part III declarations were re-confirmed for a gara. `was_stale`
   * separates the first confirmation from a re-confirmation after a change —
   * the second is the friction the reconfirmation screen exists to measure.
   */
  part_iii_reconfirmed: {
    location: AnalyticsLocation;
    declaration_count: number;
    applies_count: number;
    was_stale: boolean;
  };
  /** A file left the building. */
  dgue_exported: {
    location: AnalyticsLocation;
    version: ReportedEspdVersion;
    format: ReportedEspdFormat;
    request_known: boolean;
  };
  /**
   * A buyer's ESPD request was imported. `unmapped_criteria_count` is the read
   * on our own taxonomy coverage: a non-zero bucket means buyers are asking for
   * criteria this product cannot yet name.
   */
  dgue_request_imported: {
    location: AnalyticsLocation;
    version: ReportedEspdVersion;
    criteria_count: number;
    unmapped_criteria_count: number;
    lot_count: number;
  };
  /**
   * The dossier reached zero company-scoped gaps. The moment the compounding
   * half of the proposition pays off, and the one event here that reports on
   * the WORKSPACE rather than on a gara.
   */
  dossier_completed: {
    location: AnalyticsLocation;
    ready_field_count: number;
  };
};

export type AnalyticsEventName = keyof AnalyticsEventProps;

/** How one property is validated on its way out. */
type PropSpec =
  | { readonly kind: 'enum'; readonly values: readonly string[]; readonly allowEmpty?: true }
  | { readonly kind: 'count' }
  | { readonly kind: 'bool' }
  /** `true | false | null`, where null is a third state and not a falsy boolean. */
  | { readonly kind: 'tri_bool' };

const LOCATION: PropSpec = { kind: 'enum', values: ANALYTICS_LOCATIONS };
const COUNT: PropSpec = { kind: 'count' };
const BOOL: PropSpec = { kind: 'bool' };

/**
 * The runtime half of the catalogue. Mapped over `AnalyticsEventProps` with
 * `-?`, so TypeScript refuses a spec that omits a declared property — a new
 * property cannot be added to an event without also declaring how it is
 * validated, which is what keeps rule (2) above true over time.
 */
export const EVENT_SPECS: {
  readonly [K in AnalyticsEventName]: { readonly [P in keyof AnalyticsEventProps[K]]-?: PropSpec };
} = {
  document_coverage_shown: {
    location: LOCATION,
    coverage: { kind: 'enum', values: COVERAGE_VALUES, allowEmpty: true },
    reason: { kind: 'enum', values: COVERAGE_REASONS, allowEmpty: true },
    known_document_links: COUNT,
    extracted_documents: COUNT,
  },
  citation_opened: {
    location: LOCATION,
    surface: { kind: 'enum', values: CITATION_SURFACES },
    has_section: BOOL,
    has_page: BOOL,
  },
  eligibility_gap_shown: {
    location: LOCATION,
    gap_count: COUNT,
    blocking: BOOL,
    unknown_count: COUNT,
    coverage: { kind: 'enum', values: COVERAGE_VALUES, allowEmpty: true },
    reason: { kind: 'enum', values: COVERAGE_REASONS, allowEmpty: true },
  },
  go_no_go_recommended: {
    location: LOCATION,
    recommendation: { kind: 'enum', values: VERDICTS, allowEmpty: true },
    caveat_count: COUNT,
    authoritative_requirement_count: COUNT,
    requirement_count: COUNT,
  },
  go_no_go_overridden: {
    location: LOCATION,
    from: { kind: 'enum', values: VERDICTS, allowEmpty: true },
    to: { kind: 'enum', values: DECISIONS },
    blocking_gap_count: COUNT,
  },
  bid_decision_recorded: {
    location: LOCATION,
    decision: { kind: 'enum', values: DECISIONS },
    recommendation: { kind: 'enum', values: VERDICTS, allowEmpty: true },
    overridden: BOOL,
  },
  company_dossier_field_answered: {
    location: LOCATION,
    field_category: { kind: 'enum', values: DOSSIER_FIELD_CATEGORIES, allowEmpty: true },
    prompted_by: { kind: 'enum', values: CAPTURE_PROMPTERS },
  },
  company_dossier_question_skipped: {
    location: LOCATION,
    field_category: { kind: 'enum', values: DOSSIER_FIELD_CATEGORIES, allowEmpty: true },
  },
  requirement_confirmed: {
    location: LOCATION,
    kind: { kind: 'enum', values: REQUIREMENT_KINDS, allowEmpty: true },
    source: { kind: 'enum', values: REQUIREMENT_SOURCES, allowEmpty: true },
    had_excerpt: BOOL,
    had_citation: BOOL,
  },
  tender_criteria_viewed: {
    location: LOCATION,
    criteria_count: COUNT,
    weighted_criteria_count: COUNT,
    grid_usable: { kind: 'tri_bool' },
    lot_count: COUNT,
  },
  repair_chat_opened: {
    location: LOCATION,
    seeded_from: { kind: 'enum', values: CHAT_SEEDS },
  },
  dgue_readiness_viewed: {
    location: LOCATION,
    ready: BOOL,
    ready_field_count: COUNT,
    gap_count: COUNT,
    company_gap_count: COUNT,
    bid_gap_count: COUNT,
    declarations_complete: BOOL,
    declarations_confirmed: BOOL,
    request_known: BOOL,
  },
  dgue_gap_filled: {
    location: LOCATION,
    scope: { kind: 'enum', values: GAP_SCOPES, allowEmpty: true },
    reason: { kind: 'enum', values: GAP_REASONS, allowEmpty: true },
  },
  part_iii_reconfirmed: {
    location: LOCATION,
    declaration_count: COUNT,
    applies_count: COUNT,
    was_stale: BOOL,
  },
  dgue_exported: {
    location: LOCATION,
    version: { kind: 'enum', values: ESPD_VERSIONS },
    format: { kind: 'enum', values: ESPD_FORMATS },
    request_known: BOOL,
  },
  dgue_request_imported: {
    location: LOCATION,
    version: { kind: 'enum', values: ESPD_VERSIONS, allowEmpty: true },
    criteria_count: COUNT,
    unmapped_criteria_count: COUNT,
    lot_count: COUNT,
  },
  dossier_completed: {
    location: LOCATION,
    ready_field_count: COUNT,
  },
};

/** Every event name in the catalogue, for iteration in tests and dashboards. */
export const ANALYTICS_EVENT_NAMES = Object.keys(EVENT_SPECS) as readonly AnalyticsEventName[];

/** The only value shapes that ever leave this module. */
export type AnalyticsPropValue = string | number | boolean | null;

function scrub(spec: PropSpec, value: unknown): AnalyticsPropValue {
  switch (spec.kind) {
    case 'enum': {
      if (typeof value !== 'string') {
        return INVALID_VALUE;
      }
      if (value === '') {
        return spec.allowEmpty ? '' : INVALID_VALUE;
      }
      return spec.values.includes(value) ? value : INVALID_VALUE;
    }
    case 'count': {
      // A count is a count: a NaN, an Infinity or a negative is a bug upstream,
      // and 0 is the only reading of it that cannot mislead a rate.
      return typeof value === 'number' && Number.isFinite(value)
        ? Math.max(0, Math.trunc(value))
        : 0;
    }
    case 'bool':
      return value === true;
    case 'tri_bool':
      // Absent stays absent. Coercing it to false would report "we know the grid
      // is unusable" for a notice nobody has read.
      return value === null || value === undefined ? null : value === true;
  }
}

/**
 * Build the payload PostHog will receive for `name`.
 *
 * Iterates the SPEC, never the caller's object — an undeclared key is not
 * copied, so no free text can ride along even from a caller that has cast its
 * way past the types.
 */
export function analyticsEventProps<K extends AnalyticsEventName>(
  name: K,
  props: AnalyticsEventProps[K],
): Record<string, AnalyticsPropValue> {
  const spec = EVENT_SPECS[name] as Record<string, PropSpec>;
  const source = props as Record<string, unknown>;
  const out: Record<string, AnalyticsPropValue> = {};
  for (const [key, propSpec] of Object.entries(spec)) {
    out[key] = scrub(propSpec, source[key]);
  }
  return out;
}

/**
 * The minimum a capture target has to offer. Typed as a `Pick` of PostHog rather
 * than `PostHog` so the nullable client from `usePostHog()`/`getAnalytics()` and
 * a test double both satisfy it.
 */
export type CaptureSink = Pick<PostHog, 'capture'> | null | undefined;

/**
 * Capture a catalogued event. Unconditionally — consent gating is PostHog's job
 * (`opt_out_capturing_by_default: true`), and an `if (hasConsent)` here would be
 * both redundant and wrong.
 */
export function captureAnalyticsEvent<K extends AnalyticsEventName>(
  sink: CaptureSink,
  name: K,
  props: AnalyticsEventProps[K],
): void {
  sink?.capture(name, analyticsEventProps(name, props));
}
