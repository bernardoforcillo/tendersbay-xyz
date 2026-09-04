/**
 * The ESPD criterion and part labels, in all 24 EU languages, as the Union
 * publishes them.
 *
 * These are NOT translations this project wrote. They are extracted from the
 * official ESPD-EDM authority tables (`ExclusionGround.gc`,
 * `SelectionCriterion.gc` in https://github.com/OP-TED/espd-edm, © European
 * Union, EUPL-1.2) by `scripts/import-espd-labels.mjs`.
 *
 * Why bother, rather than translating twenty-three exclusion grounds ourselves:
 * these are the exact phrases a contracting authority's own form uses. An
 * operator reading "Grave professional misconduct" in this app and "Gravi
 * illeciti professionali" on the portal has to work out that they are the same
 * question. Using the Union's own wording removes that step in every language,
 * and it is the wording their lawyer will recognise.
 *
 * Regenerate with:
 *
 *   git clone https://github.com/OP-TED/espd-edm /tmp/espd-edm
 *   node scripts/import-espd-labels.mjs --edm /tmp/espd-edm
 */
export const OFFICIAL_CODES = {
  // Part III.A–C exclusion grounds.
  'iii.a.participation_criminal_organisation': 'exg-crim-part',
  'iii.a.corruption': 'exg-crim-corrpt',
  'iii.a.fraud': 'exg-crim-fraud',
  'iii.a.terrorist_offences': 'exg-crim-terror',
  'iii.a.money_laundering': 'exg-crim-laund',
  'iii.a.child_labour_human_trafficking': 'exg-crim-traffick',
  'iii.b.payment_of_taxes': 'exg-pmt-bre-tax',
  'iii.b.payment_of_social_security': 'exg-pmt-bre-ssc',
  'iii.c.breach_environmental_obligations': 'exg-mis-bre-env-law',
  'iii.c.breach_social_obligations': 'exg-mis-bre-soc-law',
  'iii.c.breach_labour_obligations': 'exg-mis-bre-lab-law',
  'iii.c.bankruptcy': 'exg-sitn-bankr',
  'iii.c.insolvency': 'exg-sitn-insolvency',
  'iii.c.arrangement_with_creditors': 'exg-sitn-cred-arran',
  'iii.c.analogous_situation': 'exg-sitn-other',
  'iii.c.assets_administered_by_liquidator': 'exg-sitn-liq-admin',
  'iii.c.business_activities_suspended': 'exg-sitn-as-susp',
  'iii.c.grave_professional_misconduct': 'exg-mis-misconduct',
  'iii.c.agreements_distorting_competition': 'exg-mis-distortion',
  'iii.c.conflict_of_interest': 'exg-mis-partic-confl',
  'iii.c.involvement_in_preparation': 'exg-mis-prep-confl',
  'iii.c.early_termination': 'exg-mis-sanction',
  'iii.c.misrepresentation': 'exg-mis-misrepresent',
  'iii.d.purely_national_grounds': 'exg-natl',
  // Part IV selection criteria.
  'iv.a.enrolment_professional_register': 'slc-suit-reg-prof',
  'iv.a.enrolment_trade_register': 'slc-suit-reg-trade',
  'iv.a.other_registration': 'slc-suit-auth-mbrshp',
  'iv.b.general_yearly_turnover': 'slc-stand-to-gen',
  'iv.b.specific_yearly_turnover': 'slc-stand-to-spec',
  'iv.c.references': 'slc-abil-ref-work',
  'iv.c.average_annual_manpower': 'slc-abil-staff-yrly-avg-mp',
  'iv.d.quality_assurance_certificates': 'slc-sche-qu-cert-indep',
  'iv.d.environmental_management_certificates': 'slc-sche-env-cert-indep',
};

/**
 * The Part III headings, as the Union words them. Only the three the dossier
 * form groups its twenty-three grounds under are mapped: a heading nobody
 * renders would be twenty-four translations of dead weight, and the tables
 * carry the rest whenever a screen needs them.
 *
 * III.C is `exg-mis` and not `exg-sitn`: Regulation 2016/7 puts insolvency,
 * conflicts of interest and professional misconduct in one section, which is
 * the group `exg-mis` names — `exg-sitn` is the narrower insolvency subset.
 */
export const OFFICIAL_PART_CODES = {
  'III.A': 'exg-crim',
  'III.B': 'exg-pmt',
  'III.C': 'exg-mis',
};

/**
 * Criteria this product names itself, because the ESPD models them as document
 * structure rather than as criteria with a published label.
 */
export const OWN_CRITERIA = [
  'i.procedure',
  'i.lots',
  'ii.a.identity',
  'ii.b.representatives',
  'ii.c.reliance',
  'ii.d.subcontracting',
  'iv.c.soa_attestation',
];
