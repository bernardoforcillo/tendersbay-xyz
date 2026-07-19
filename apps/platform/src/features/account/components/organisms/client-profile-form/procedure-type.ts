/**
 * The fixed procurement procedure types a client profile may filter on —
 * mirrors clientprofile.go's `validProcedureTypes` exactly. Unlike NUTS
 * regions (a large hierarchical taxonomy with no enumerable list in this
 * frontend), this is a small closed set, so it renders as toggle-button
 * chips — the same pattern as sectors/countries.
 */
export const PROCEDURE_TYPES = [
  'open',
  'restricted',
  'negotiated',
  'competitive_dialogue',
  'innovation_partnership',
  'other',
] as const;

export type ProcedureType = (typeof PROCEDURE_TYPES)[number];
