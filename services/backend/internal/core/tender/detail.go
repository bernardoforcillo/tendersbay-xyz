package tender

import (
	"errors"
	"time"
)

// ErrTenderNotFound is returned when no tender has the given id.
var ErrTenderNotFound = errors.New("tender: not found")

// Document is one attached notice/document link.
type Document struct {
	URL  string
	Type string
}

// Lot is one procurement lot.
type Lot struct {
	Ref      string
	Title    string
	CPV      string
	Value    *int64
	Currency string
	Deadline *time.Time
}

// TenderDetail is the full single-tender view (superset of Tender's fields).
type TenderDetail struct {
	ID            string
	Title         string
	BuyerName     string
	BuyerID       string
	Status        string
	ProcedureType string
	Country       string
	NUTS          string
	Language      string
	CPV           string
	CPVSecondary  []string
	Value         *int64
	Currency      string
	PublishedAt   *time.Time
	Deadline      *time.Time
	Source        string
	SourceRef     string
	SourceURL     string
	Documents     []Document
	Lots          []Lot
}

// TenderRef is one sitemap entry.
type TenderRef struct {
	ID      string
	Lastmod string
}
