package webdoc

import "github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/core/retrieve"

// The per-tender request budget, re-exported from the core layer.
//
// The per-host pacer in fetch.go bounds how FAST we talk to one server; this
// bounds how MUCH we ask of it for one tender. They are different promises and
// neither implies the other: a framework agreement whose listing publishes three
// hundred attachments would be perfectly paced and still be three hundred
// requests aimed at one comune for one row.
//
// It is DEFINED in internal/core/retrieve and only named here, because it is
// spent there. This package cannot see a tender at all — it fetches one address
// at a time and has no idea which row asked for it — so the only place these
// caps can be enforced is the loop that walks a tender's files. What is here is
// the pointer, so that a reader of the fetcher finds the per-tender bounds
// alongside the per-response and per-host ones instead of having to know they
// exist somewhere else.
const (
	// MaxFilesPerTender caps downloads for one tender. Fifteen covers the
	// observed Italian shape of a procedure — bando, disciplinare, capitolato,
	// DGUE, schema di contratto, patto di integrità, planimetrie, and a handful
	// of modelli — with room to spare. A listing with three hundred files is
	// not a reason to make three hundred requests; it is a reason to record
	// that there were three hundred and read the fifteen that matter.
	MaxFilesPerTender = retrieve.MaxFilesPerTender

	// MaxTenderBytes caps total downloaded bytes for one tender. It is
	// deliberately not MaxFilesPerTender × maxDocumentBytes (300 MiB): that
	// product is an arithmetic possibility, not a budget anyone would choose to
	// spend on one row, and the pod it runs in is sized for a working set an
	// order of magnitude smaller.
	MaxTenderBytes = retrieve.MaxTenderBytes
)

// Budget tracks one tender's consumption against those two caps. It is not safe
// for concurrent use, which matches how it is meant to be used: one tender's
// files are fetched serially, because they are all behind one host's single
// request slot anyway.
type Budget = retrieve.Budget
