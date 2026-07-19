// Package codice parses Spain's PLACSP CODICE/UBL contract-folder documents —
// the payload that sits inline inside each entry of the PLACSP syndication
// ATOM feed — into a neutral Document, and maps that Document onto the shared
// tender.Tender model (see map.go).
//
// CODICE is Spain's national UBL profile: every element lives in one of the
// urn:dgpe:… CODICE namespaces, carried by the prefixes cbc:/cac:/cbc-place-ext:/
// cac-place-ext:. Go's encoding/xml matches a struct tag by local name alone
// when the tag names no namespace, so this package tags every field by local
// name and leans on the element hierarchy to disambiguate collisions (e.g. the
// many <cbc:Name> elements). A useful consequence: a folder lifted out of the
// feed without the feed's xmlns declarations still decodes — encoding/xml
// leaves an unbound prefix as-is and local-name matching ignores it.
package codice

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Document is the subset of a CODICE ContractFolderStatus this pipeline needs,
// already normalised: EstimatedValue is minor units, SubmissionDeadline is a
// resolved time. Optional elements absent from the source leave their zero
// value (nil pointers, "" strings, nil CPV).
type Document struct {
	ContractFolderID   string
	StatusCode         string     // cbc-place-ext:ContractFolderStatusCode (e.g. "EV")
	Title              string     // cac:ProcurementProject/cbc:Name
	CPV                []string   // every ItemClassificationCode, in document order
	EstimatedValue     *int64     // minor units; nil when the folder carries no amount
	Currency           string     // ISO-4217, from the amount's currencyID attribute
	SubmissionDeadline *time.Time // TenderSubmissionDeadlinePeriod EndDate(+EndTime)
	BuyerName          string     // the direct LocatedContractingParty's Party name
	NUTS               string     // RealizedLocation/cbc:CountrySubentityCode
	Raw                []byte     // untouched CODICE payload
}

// contractFolderStatus is the decode target rooted at a ContractFolderStatus
// element. Every xml tag is a bare local name; matching ignores the namespace
// prefix.
type contractFolderStatus struct {
	XMLName          xml.Name           `xml:"ContractFolderStatus"`
	ContractFolderID string             `xml:"ContractFolderID"`
	StatusCode       string             `xml:"ContractFolderStatusCode"`
	Party            locatedParty       `xml:"LocatedContractingParty"`
	Project          procurementProject `xml:"ProcurementProject"`
	Process          tenderingProcess   `xml:"TenderingProcess"`
}

// locatedParty reads only the buyer's own Party name. It deliberately does not
// declare ParentLocatedParty, so the buyer name can never bleed up the
// contracting-org hierarchy (Ayuntamiento → Provincia → Comunidad → …).
type locatedParty struct {
	Party struct {
		PartyName struct {
			Name string `xml:"Name"`
		} `xml:"PartyName"`
	} `xml:"Party"`
}

type procurementProject struct {
	Name            string           `xml:"Name"`
	Budget          budgetAmount     `xml:"BudgetAmount"`
	Classifications []classification `xml:"RequiredCommodityClassification"`
	Location        realizedLocation `xml:"RealizedLocation"`
}

type classification struct {
	Code string `xml:"ItemClassificationCode"`
}

// budgetAmount carries the three amounts a CODICE BudgetAmount may hold.
type budgetAmount struct {
	EstimatedOverall   amount `xml:"EstimatedOverallContractAmount"`
	TotalAmount        amount `xml:"TotalAmount"`
	TaxExclusiveAmount amount `xml:"TaxExclusiveAmount"`
}

type amount struct {
	Value    string `xml:",chardata"`
	Currency string `xml:"currencyID,attr"`
}

type realizedLocation struct {
	CountrySubentityCode string `xml:"CountrySubentityCode"`
}

// tenderingProcess reads only the submission deadline. A CODICE TenderingProcess
// also holds a DocumentAvailabilityPeriod with its own EndDate/EndTime; naming
// TenderSubmissionDeadlinePeriod explicitly keeps the two from being confused.
type tenderingProcess struct {
	Deadline struct {
		EndDate string `xml:"EndDate"`
		EndTime string `xml:"EndTime"`
	} `xml:"TenderSubmissionDeadlinePeriod"`
}

// Parse decodes one CODICE contract-folder payload. Malformed XML returns an
// error; the caller (placspapi) skips-and-logs a bad entry rather than failing
// the whole batch. A well-formed payload with missing optional elements decodes
// with those fields left at their zero value.
func Parse(payload []byte) (Document, error) {
	var cfs contractFolderStatus
	if err := xml.Unmarshal(payload, &cfs); err != nil {
		return Document{}, fmt.Errorf("codice: parse contract folder: %w", err)
	}

	doc := Document{
		ContractFolderID:   strings.TrimSpace(cfs.ContractFolderID),
		StatusCode:         strings.TrimSpace(cfs.StatusCode),
		Title:              strings.TrimSpace(cfs.Project.Name),
		BuyerName:          strings.TrimSpace(cfs.Party.Party.PartyName.Name),
		NUTS:               strings.TrimSpace(cfs.Project.Location.CountrySubentityCode),
		SubmissionDeadline: parseDeadline(cfs.Process.Deadline.EndDate, cfs.Process.Deadline.EndTime),
		Raw:                payload,
	}
	for _, c := range cfs.Project.Classifications {
		if code := strings.TrimSpace(c.Code); code != "" {
			doc.CPV = append(doc.CPV, code)
		}
	}
	amt := pickAmount(cfs.Project.Budget)
	doc.EstimatedValue = parseMinorUnits(amt.Value)
	doc.Currency = strings.TrimSpace(amt.Currency)
	return doc, nil
}

// pickAmount prefers the tax-exclusive budget (the "importe" PLACSP headlines
// in each entry summary — verified against the fixture: 70850 = TaxExclusive =
// EstimatedOverall, while TotalAmount 85728.5 includes tax), falling back to
// the total, then the estimated-overall amount.
func pickAmount(b budgetAmount) amount {
	switch {
	case strings.TrimSpace(b.TaxExclusiveAmount.Value) != "":
		return b.TaxExclusiveAmount
	case strings.TrimSpace(b.TotalAmount.Value) != "":
		return b.TotalAmount
	default:
		return b.EstimatedOverall
	}
}

// parseMinorUnits converts a decimal amount string ("70850", "206367.77",
// "85728.5" — CODICE amounts have inconsistent fractional-digit counts, like
// TED's) into minor units, without the precision loss float64 multiplication
// would introduce. Returns nil for an empty or malformed string.
func parseMinorUnits(s string) *int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	whole, frac, _ := strings.Cut(s, ".")
	switch {
	case len(frac) == 0:
		frac = "00"
	case len(frac) == 1:
		frac += "0"
	case len(frac) > 2:
		frac = frac[:2]
	}
	n, err := strconv.ParseInt(whole+frac, 10, 64)
	if err != nil {
		return nil
	}
	if neg {
		n = -n
	}
	return &n
}

// parseDeadline combines CODICE's separate EndDate ("2026-05-13") and optional
// EndTime ("23:59:00") into one time.Time. Unlike TED's, these carry no UTC
// offset on the wire, so the result is a naive instant read as UTC (see the
// package caveat). An EndDate shorter than a date, or an unparseable
// combination, yields nil.
func parseDeadline(dateStr, timeStr string) *time.Time {
	dateStr = strings.TrimSpace(dateStr)
	if len(dateStr) < 10 {
		return nil
	}
	date := dateStr[:10]
	clock := "00:00:00"
	if t := strings.TrimSpace(timeStr); len(t) >= 8 {
		clock = t[:8]
	}
	parsed, err := time.Parse("2006-01-02T15:04:05", date+"T"+clock)
	if err != nil {
		return nil
	}
	return &parsed
}
