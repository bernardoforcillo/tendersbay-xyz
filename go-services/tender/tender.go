// Package tender defines the normalized tender domain model shared between
// services/ingestion (the writer) and backend (a future reader). It is pure
// data — no methods beyond what's needed for encoding, no DB tags, no I/O —
// so importing it never pulls in a database driver or any write-path
// dependency.
package tender

import (
	"encoding/json"
	"time"
)

// Tender is the canonical shape any provider produces.
type Tender struct {
	Source        string // e.g. "ted", "it-mepa" — the provider's Name()
	SourceRef     string // stable per-provider id (notice number, etc.)
	Title         string
	Buyer         Buyer
	Status        Status   // normalized lifecycle status — see below
	ProcedureType string   // e.g. "open", "restricted", "negotiated", "competitive-dialogue"
	Language      string   // ISO 639-1 of the original text ("it", "fr", …); "" when unknown
	Country       string   // ISO-3166 alpha-2
	NUTS          string   // NUTS region code; "" when unknown
	CPV           string   // main/primary Common Procurement Vocabulary code
	CPVSecondary  []string // additional CPV codes; nil when none
	Value         *int64   // minor units; nil when unknown
	Currency      string   // ISO-4217; "" when unknown
	PublishedAt   *time.Time
	Deadline      *time.Time
	Documents     []Document      // notice PDF, technical specs, corrigenda, …
	Lots          []Lot           // procurement lots; nil for single-lot tenders
	Raw           json.RawMessage // untouched provider payload
}

// Buyer identifies the contracting authority. ID is a stable
// registration/VAT number when the provider exposes one, letting
// cross-tender analysis follow the same buyer over time; it is "" when
// unknown.
type Buyer struct {
	Name string
	ID   string
}

// Document is one file attached to a notice (the notice itself, technical
// specs, a corrigendum, …).
type Document struct {
	URL  string
	Type string // e.g. "notice", "spec", "corrigendum"
}

// Lot is one procurement lot within a multi-lot tender, carrying its own
// scope, CPV, value, and deadline. Single-lot tenders have no Lot entries —
// their scope lives directly on Tender's own CPV/Value/Deadline.
type Lot struct {
	Ref      string // lot number/identifier within the tender
	Title    string
	CPV      string
	Value    *int64
	Currency string
	Deadline *time.Time
}

// Status is the normalized tender lifecycle, shared across all providers.
// Each connector maps its source's native status text onto this fixed enum
// inside its Fetch implementation; a provider that can't confidently map a
// native status emits StatusUnknown rather than guessing.
type Status string

const (
	StatusOpen      Status = "open"      // accepting bids
	StatusAwarded   Status = "awarded"   // contract awarded
	StatusCancelled Status = "cancelled" // withdrawn/annulled
	StatusClosed    Status = "closed"    // deadline passed, no award recorded
	StatusUnknown   Status = "unknown"   // provider status didn't map cleanly
)
