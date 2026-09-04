/**
 * The ESPD/DGUE vocabularies the UI renders by code.
 *
 * The wire carries stable strings — `espd.proto` keeps parts, scopes, reasons
 * and criterion keys as strings rather than proto enums, so a Go constant
 * rename is not a wire break — which means the UI owns all 24 renderings. These
 * arrays are the mechanical link between a Go constant and its translations:
 * the locale test walks them, so a criterion added in Go without copy fails in
 * every locale at once instead of printing `iii.c.bankruptcy` at a person about
 * to sign a legal declaration.
 */

/**
 * Where a gap is fixed. This is the axis the readiness card is built around:
 * `company` gaps are answered ONCE and close for every future gara, `bid` gaps
 * belong to this one. Mixing them into a single to-do list would hide the
 * compounding half of the product's proposition.
 */
export const GAP_SCOPES = ['company', 'bid'] as const;
export type GapScope = (typeof GAP_SCOPES)[number];

/** Why a field is empty. Each reason wants a different sentence, not a colour. */
export const GAP_REASONS = ['missing', 'not_authoritative', 'stale'] as const;
export type GapReason = (typeof GAP_REASONS)[number];

/** The two ESPD data models an export can target. */
export const ESPD_VERSIONS = ['edm_2_1_1', 'edm_4'] as const;
export type EspdVersion = (typeof ESPD_VERSIONS)[number];

export const ESPD_FORMATS = ['xml', 'pdf'] as const;
export type EspdFormat = (typeof ESPD_FORMATS)[number];

/**
 * The Part III.A–C exclusion grounds, in document order.
 *
 * 1:1 with `espd.ExclusionCriteria()` in Go, and the reconfirmation screen is
 * built from this list rather than from whatever the server happened to return:
 * a ground the server stopped sending would silently vanish from the screen a
 * person signs, which is the one place a silent omission is unacceptable.
 */
export const EXCLUSION_CRITERIA = [
  'iii.a.participation_criminal_organisation',
  'iii.a.corruption',
  'iii.a.fraud',
  'iii.a.terrorist_offences',
  'iii.a.money_laundering',
  'iii.a.child_labour_human_trafficking',
  'iii.b.payment_of_taxes',
  'iii.b.payment_of_social_security',
  'iii.c.breach_environmental_obligations',
  'iii.c.breach_social_obligations',
  'iii.c.breach_labour_obligations',
  'iii.c.bankruptcy',
  'iii.c.insolvency',
  'iii.c.arrangement_with_creditors',
  'iii.c.analogous_situation',
  'iii.c.assets_administered_by_liquidator',
  'iii.c.business_activities_suspended',
  'iii.c.grave_professional_misconduct',
  'iii.c.agreements_distorting_competition',
  'iii.c.conflict_of_interest',
  'iii.c.involvement_in_preparation',
  'iii.c.early_termination',
  'iii.c.misrepresentation',
] as const;
export type ExclusionCriterion = (typeof EXCLUSION_CRITERIA)[number];

/**
 * Every criterion the UI may have to name: the exclusion grounds plus the
 * structural and selection criteria the composed document carries.
 */
export const ESPD_CRITERIA = [
  ...EXCLUSION_CRITERIA,
  'iii.d.purely_national_grounds',
  'i.procedure',
  'i.lots',
  'ii.a.identity',
  'ii.b.representatives',
  'ii.c.reliance',
  'ii.d.subcontracting',
  'iv.a.enrolment_professional_register',
  'iv.a.enrolment_trade_register',
  'iv.a.other_registration',
  'iv.b.general_yearly_turnover',
  'iv.b.specific_yearly_turnover',
  'iv.c.references',
  'iv.c.average_annual_manpower',
  'iv.c.soa_attestation',
  'iv.d.quality_assurance_certificates',
  'iv.d.environmental_management_certificates',
] as const;

/**
 * A Part III.D answer is keyed `iii.d.<country>.<national code>` — national law
 * defines the catalogue, so the set is open and cannot be enumerated here. The
 * UI names those with the country and the raw national code, which is what the
 * operator's own lawyer used when they answered.
 */
export function isNationalGround(criterion: string): boolean {
  return criterion.startsWith('iii.d.') && criterion !== 'iii.d.purely_national_grounds';
}

/** `iii.d.it.art94.c1` → `{ country: 'IT', code: 'art94.c1' }`. */
export function parseNationalGround(criterion: string): { country: string; code: string } {
  const rest = criterion.slice('iii.d.'.length);
  const dot = rest.indexOf('.');
  if (dot < 0) return { country: rest.toUpperCase(), code: '' };
  return { country: rest.slice(0, dot).toUpperCase(), code: rest.slice(dot + 1) };
}

/**
 * The Part III headings the dossier groups its twenty-three grounds under —
 * the same three sections the official form has. Grouping is not decoration
 * here: twenty-three legal grounds in one flat list is a wall, and the operator
 * reading it has almost certainly seen the same three headings on the
 * contracting authority's own form.
 */
export const DECLARATION_PARTS = ['III.A', 'III.B', 'III.C'] as const;
export type DeclarationPart = (typeof DECLARATION_PARTS)[number];

/**
 * `iii.c.bankruptcy` → `III.C`. Derived from the key rather than from a second
 * table, so a ground added in Go lands under the right heading by itself.
 */
export function partOfCriterion(criterion: string): string {
  const match = /^(i|ii|iii|iv|v|vi)\.([a-d])\./.exec(criterion);
  if (match?.[1] && match[2]) return `${match[1].toUpperCase()}.${match[2].toUpperCase()}`;
  return (criterion.split('.')[0] ?? '').toUpperCase();
}
