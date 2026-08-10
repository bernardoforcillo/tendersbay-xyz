/**
 * Company/eligibility domain constants and the view shapes the scheda gara reads.
 *
 * Same contract as `~/features/tenders/constants`: the arrays exist so
 * `src/assets/locales/company-keys.test.ts` can assert a translation for every
 * member in every one of the 24 locales. A gap status or a remedy kind that
 * renders as its raw snake_case code is not a cosmetic failure — it is the
 * product telling a user something it cannot say.
 *
 * These constants are deliberately NOT proto enums on the wire (see
 * `proto/company/v1/company.proto`), so they are stable strings on both sides and
 * a rename is a visible, greppable break rather than a silent wire change.
 */

/**
 * `unknown` is NOT `missing`. Missing means we asked and the answer was no;
 * unknown means we never asked — and unknown is the just-in-time capture
 * trigger. Collapsing them would delete the only signal that says "ask".
 */
export const GAP_STATUSES = [
  'met',
  'missing',
  'shortfall',
  'expiring',
  'expired',
  'unknown',
] as const;
export type GapStatus = (typeof GAP_STATUSES)[number];

/**
 * How a gap can be closed. A blocking gap with no remedy line reads as
 * unsolvable, which for an Italian SOA or turnover gap is usually false —
 * avvalimento and RTI exist precisely for it.
 */
export const REMEDY_KINDS = [
  'provide_fact',
  'renew',
  'upgrade',
  'avvalimento',
  'rti',
  'subcontract',
] as const;
export type RemedyKind = (typeof REMEDY_KINDS)[number];

/**
 * What the verdict does not know. Silence about these is how ignorance gets read
 * as a clean bill, so every caveat the server raises is rendered.
 */
export const CAVEATS = [
  'no_requirements_captured',
  'award_grid_only',
  'unconfirmed_extraction',
  'open_questions',
  'documents_unread',
] as const;
export type Caveat = (typeof CAVEATS)[number];

/**
 * `insufficient_data` is a verdict, not the absence of one. A bid whose decision
 * record carries no recommendation at all is a different fact again — see
 * `bid-decision`.
 */
export const VERDICTS = ['insufficient_data', 'go', 'no_go'] as const;
export type Verdict = (typeof VERDICTS)[number];

/**
 * Requirement kinds double as the routing key for just-in-time capture: the UI
 * picks its prompt and its destination RPC off `requirement.kind`, which is why
 * the server's `CaptureQuestion.Field` never needed to reach the wire.
 */
export const REQUIREMENT_KINDS = [
  'soa',
  'certification',
  'turnover',
  'past_contracts',
  'headcount',
  'registration',
  'other',
] as const;
export type RequirementKind = (typeof REQUIREMENT_KINDS)[number];

/** Where a captured requirement came from. Drives the confirm affordance. */
export const REQUIREMENT_SOURCES = [
  'user_stated',
  'document_excerpt',
  'award_criteria',
  'agent_inferred',
] as const;
export type RequirementSource = (typeof REQUIREMENT_SOURCES)[number];

/** How a dossier fact came to be believed. The whole reason `Attribution` exists. */
export const PROVENANCES = ['user_stated', 'user_confirmed', 'agent_inferred', 'imported'] as const;
export type Provenance = (typeof PROVENANCES)[number];

/** SOA general categories (opere generali). */
export const SOA_OG_CATEGORIES = Array.from({ length: 13 }, (_, i) => `OG${i + 1}`);
/** SOA specialised categories (opere specializzate). */
export const SOA_OS_CATEGORIES = Array.from({ length: 35 }, (_, i) => `OS${i + 1}`);
export const SOA_CATEGORIES = [...SOA_OG_CATEGORIES, ...SOA_OS_CATEGORIES];

/**
 * Classifiche in ascending order. `III-bis` and `IV-bis` are real intermediate
 * bands, not typos — an operator holding III-bis does not hold IV.
 */
export const SOA_CLASSIFICHE = [
  'I',
  'II',
  'III',
  'III-bis',
  'IV',
  'IV-bis',
  'V',
  'VI',
  'VII',
  'VIII',
] as const;

export const CERTIFICATION_STANDARDS = [
  'iso_9001',
  'iso_14001',
  'iso_45001',
  'iso_27001',
  'iso_37001',
  'iso_50001',
  'sa_8000',
  'uni_pdr_125',
  'iso_20121',
  'emas',
  'other',
] as const;

export const REGISTRATION_KINDS = [
  'white_list',
  'cciaa',
  'albo_gestori_ambientali',
  'mepa',
  'sistema_qualificazione',
  'albo_professionale',
  'anac',
  'other',
] as const;

export const PAST_CONTRACT_ROLES = [
  'principal',
  'subcontractor',
  'rti_mandataria',
  'rti_mandante',
] as const;

export const LEGAL_FORMS = [
  'srl',
  'srls',
  'spa',
  'sapa',
  'sas',
  'snc',
  'ditta_individuale',
  'cooperativa',
  'consorzio',
  'societa_semplice',
  'ente_pubblico',
  'other',
] as const;

/**
 * The subset of `company.v1.TenderRequirement` the scheda gara renders.
 *
 * `requirementText` and `excerpt` are as-published prose in the gara's own
 * language and are rendered verbatim — translating them would be translating
 * data.
 */
export type RequirementView = {
  id: string;
  tenderId?: string;
  lotRef?: string;
  kind: string;
  requirementText: string;
  blocking?: boolean;
  source: string;
  excerpt?: string;
  documentUrl?: string;
  page?: string;
  section?: string;
  /** "" = unconfirmed. An unconfirmed extraction may raise a question, never a verdict. */
  confirmedBy?: string;
  confirmedAt?: string;
};

/**
 * The subset of `company.v1.EligibilityGap` the scheda gara renders.
 *
 * `held` is what the dossier actually says, carried so the UI leads with the
 * FACT rather than with a verdict. `question` is deliberately absent from this
 * view: it is model-facing Italian prose, and a 24-locale UI builds its own
 * prompt from `requirement.kind`.
 */
export type GapView = {
  requirement?: RequirementView;
  status: string;
  blocking?: boolean;
  held?: string;
  remedy?: string;
};

/**
 * The subset of `company.v1.EligibilityAssessment` the scheda gara renders.
 *
 * `caveats` is optional because it is a Phase 4 transport addition: a generated
 * message without the field still satisfies this type, and so does one with it.
 */
export type AssessmentView = {
  tenderId?: string;
  gaps: GapView[];
  requirementCount?: number;
  authoritativeRequirementCount?: number;
  blockingGapCount?: number;
  unknownGapCount?: number;
  documentCoverage?: string;
  documentReason?: string;
  verdict: string;
  lotRef?: string;
  evaluatedAt?: string;
  caveats?: string[];
};

/** The subset of `company.v1.Attribution` the provenance mark renders. */
export type AttributionView = {
  provenance?: string;
  statedAt?: string;
  promptedBy?: string;
};
