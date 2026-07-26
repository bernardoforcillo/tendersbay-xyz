package bid

// checklistTemplate returns the ordered ESPD/DGUE seeds for a new bid. v1 emits
// a single baseline set — Part II (operator information), Part III (exclusion
// grounds), Part IV (selection criteria), and a concluding declaration —
// regardless of procedureType/cpv; an empty/unknown procedureType falls back to
// this same baseline. Both args are the forward hook for per-sector variation
// later (added without a schema change).
//
// SectionCode/ItemCode are STABLE, DOT-FREE i18n code stems, never label text.
// They are the canonical contract shared verbatim with the bid i18n namespace
// (P2.3: bid.checklist.sections.<SectionCode> and bid.checklist.items.<ItemCode>)
// and the frontend renderer (P2.4). Item codes MUST stay dot-free: i18next's
// default keySeparator is '.', so a dotted code would resolve against a nested
// path the flat items map does not have. If you change any code here, change it
// identically in P2.3's 24 locale payloads and bid-keys.test.ts.
func checklistTemplate(procedureType, cpv string) []ChecklistItemSeed {
	return []ChecklistItemSeed{
		{SectionCode: "part_ii", ItemCode: "identification", Required: true, Position: 0},
		{SectionCode: "part_ii", ItemCode: "sme_status", Required: false, Position: 1},
		{SectionCode: "part_ii", ItemCode: "representation", Required: true, Position: 2},
		{SectionCode: "part_iii", ItemCode: "criminal_convictions", Required: true, Position: 3},
		{SectionCode: "part_iii", ItemCode: "tax_payments", Required: true, Position: 4},
		{SectionCode: "part_iii", ItemCode: "social_security", Required: true, Position: 5},
		{SectionCode: "part_iii", ItemCode: "insolvency", Required: true, Position: 6},
		{SectionCode: "part_iii", ItemCode: "misconduct", Required: true, Position: 7},
		{SectionCode: "part_iii", ItemCode: "conflict_interest", Required: true, Position: 8},
		{SectionCode: "part_iv", ItemCode: "suitability", Required: true, Position: 9},
		{SectionCode: "part_iv", ItemCode: "economic_standing", Required: true, Position: 10},
		{SectionCode: "part_iv", ItemCode: "technical_ability", Required: true, Position: 11},
		{SectionCode: "part_iv", ItemCode: "quality_assurance", Required: false, Position: 12},
		{SectionCode: "conclusion", ItemCode: "espd_signed", Required: true, Position: 13},
	}
}
