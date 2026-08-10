/**
 * Tender-side domain constants and the view shapes the document/criteria UI reads.
 *
 * WHY the unions live here as arrays rather than as bare TS unions: they are the
 * only mechanical link between a Go constant and its 24 renderings.
 * `src/assets/locales/tenders-keys.test.ts` iterates these arrays and asserts a
 * translation exists for every member in every locale, so adding a
 * `document.Reason` in the backend without adding copy fails the frontend suite
 * loudly instead of shipping a raw snake_case code to a user. For a coverage
 * *cause* that would not merely look unpolished — it would be a lie about
 * coverage.
 *
 * WHY the message shapes below are declared structurally instead of imported
 * from `@tendersbay/proto/tender/v1/tender_pb`: these are deliberately *view*
 * types, narrower than the wire.
 *
 *   - `CitationView` is fed from two different wire shapes — `tender.v1`'s
 *     structured `DocumentCitation` (from GetTenderPassages) and
 *     `company.v1.TenderRequirement`'s flat `document_url`/`page`/`section`
 *     strings. One renderer over one view type is what makes the degradation
 *     ladder live in exactly one place; see `citationFromRequirement` below.
 *   - Every field a Phase 4 transport addition introduces is **optional** here.
 *     A generated message that does not carry the field yet still satisfies the
 *     view type, and so does one that does. That is not laxity: the UI's honest
 *     position on `criteria`/`caveats`/`decision_*` is "absent is a real state",
 *     which is exactly what these components already have to render.
 *
 * This is not a re-validation layer at the transport boundary — protobuf is
 * still the contract (@.claude/rules/code-organization.md). Nothing here parses
 * or checks the wire; it only narrows it for rendering.
 */

/** Quantity of document text we hold. Mirrors `core/document.Coverage`. */
export const COVERAGE_VALUES = ['full', 'notice_only', 'none'] as const;
export type Coverage = (typeof COVERAGE_VALUES)[number];

/**
 * WHY the cause is a separate axis from the quantity: `notice_only` because the
 * buyer published nothing else and `notice_only` because we have not fetched
 * what they did publish are the same quantity and opposite facts. `core/document`
 * keeps them orthogonal and enforces the pairing; the UI must not flatten them.
 */
export const COVERAGE_REASONS = [
  'no_documents_published',
  'notice_not_read',
  'body_not_retrieved',
  'not_yet_extracted',
  'extraction_failed',
] as const;
export type CoverageReason = (typeof COVERAGE_REASONS)[number];

/** Award-criterion families as published in eForms. */
export const CRITERION_TYPES = ['quality', 'cost', 'price'] as const;
export type CriterionType = (typeof CRITERION_TYPES)[number];

/**
 * Where a quoted passage came from.
 *
 * `pageStartSet` and not a nullable `pageStart` is the house optional idiom, and
 * it matters more here than anywhere: `page_start` alone is `0` for both "page
 * zero" and "we never learned the page", and rendering "p. 0" under a verbatim
 * quote would undermine the one affordance this whole feature rests on. Missing
 * page/section metadata is the NORMAL case for documents extracted before the
 * extractor recorded it — not an edge case.
 */
export type CitationView = {
  documentUrl: string;
  /** Published label, e.g. "Disciplinare". Rendered verbatim — never localized. */
  documentType?: string;
  partIndex?: number;
  pageStart?: number;
  pageStartSet?: boolean;
  pageEnd?: number;
  pageEndSet?: boolean;
  /** Hierarchical numbering, e.g. ["7", "7.2"]. Empty when unknown. */
  sectionPath?: string[];
  sectionTitle?: string;
  hasTable?: boolean;
};

/** One retrieved passage of a tender document. */
export type PassageView = {
  text: string;
  truncated?: boolean;
  relevanceScore?: number;
  citation?: CitationView;
};

/**
 * What we hold for a tender, quantity and cause together.
 *
 * Availability and passages travel together on the wire and they travel together
 * through the UI for the same reason: a caller that sees an empty passage list
 * without a cause conflates "no passage matched your question" with "we hold
 * nothing but the notice PDF".
 */
export type DocumentAvailabilityView = {
  coverage: string;
  reason: string;
  noticeRead?: boolean;
  knownDocumentLinks?: number;
  extractedDocuments?: number;
  extractedParts?: number;
};

/**
 * One award criterion as published.
 *
 * `weight` is meaningless unless `weightSet`. Never coalesce absent to 0: a zero
 * weight and an absent weight are different published facts, and flattening them
 * makes "criteria published" read as "grid recoverable" — which overstates real
 * coverage by roughly 2x on Italian notices.
 */
export type AwardCriterionView = {
  /** "" = notice-level, applies to every lot. */
  lotRef: string;
  ordinal?: number;
  type: string;
  name: string;
  description?: string;
  weight?: number;
  weightSet?: boolean;
  /** "30", "30%", "30,5" — as published, so a parse failure loses comparability, not the datum. */
  weightRaw?: string;
  /** ISO 639-2/T, e.g. "ITA". Multilingual notices repeat every text field per language. */
  lang?: string;
};

/** The subset of `tender.v1.TenderDetail` the Phase 4 surfaces read. */
export type TenderCriteriaSource = {
  criteria?: AwardCriterionView[];
  /** Meaningful only if `gridUsableSet`; unset = never enriched, a third state. */
  gridUsable?: boolean;
  gridUsableSet?: boolean;
  /** RFC3339; "" = the notice's structured data was never read. */
  enrichedAt?: string;
  documentsUrl?: string;
  language?: string;
};

/** A page string as published: "14", "14-16", or "" when unknown. */
function parsePage(
  page: string,
): Pick<CitationView, 'pageStart' | 'pageStartSet' | 'pageEnd' | 'pageEndSet'> {
  const [fromRaw, toRaw] = page.split('-', 2);
  const from = Number.parseInt((fromRaw ?? '').trim(), 10);
  if (!Number.isFinite(from)) return { pageStartSet: false, pageEndSet: false };
  const to = Number.parseInt((toRaw ?? '').trim(), 10);
  return Number.isFinite(to)
    ? { pageStart: from, pageStartSet: true, pageEnd: to, pageEndSet: true }
    : { pageStart: from, pageStartSet: true, pageEndSet: false };
}

/**
 * Adapts a `company.v1.TenderRequirement`'s flat citation strings onto the same
 * view type the structured `tender.v1.DocumentCitation` produces, so a
 * requirement's evidence renders through exactly the same degradation ladder as
 * a retrieved passage's. The alternative — a second formatter for the flat shape
 * — is how "p. 0" and an empty "§" get shipped in one of the two.
 */
export function citationFromRequirement(requirement: {
  documentUrl?: string;
  page?: string;
  section?: string;
}): CitationView {
  const section = (requirement.section ?? '').trim();
  return {
    documentUrl: requirement.documentUrl ?? '',
    ...parsePage(requirement.page ?? ''),
    sectionPath: [],
    sectionTitle: section,
  };
}
