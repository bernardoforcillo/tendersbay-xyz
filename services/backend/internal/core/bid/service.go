package bid

// Service owns the bid lifecycle and the ESPD checklist. It authorizes every
// call against the workbench RBAC layer, then delegates persistence to its
// Repository — no business rules leak into the transport or adapter layers.
type Service struct {
	repo      Repository
	access    WorkbenchAccess
	fit       TenderFit
	summaries TenderSummaries
}

// NewService wires the Service. fit and summaries are BOTH taken from the single
// tenders arg (*tender.Service satisfies both ports).
func NewService(repo Repository, access WorkbenchAccess, tenders Tenders) *Service {
	return &Service{repo: repo, access: access, fit: tenders, summaries: tenders}
}
