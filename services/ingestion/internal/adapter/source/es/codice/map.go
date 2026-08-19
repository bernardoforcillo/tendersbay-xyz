package codice

import (
	"encoding/json"
	"strings"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
)

// Map normalises one CODICE Document onto tender.Tender for the es-placsp
// source. SourceRef is the ContractFolderID — Spain's stable per-folder id, so
// a later status change to the same folder lands as an update to the same row.
// Value is populated (CODICE carries the budget amount), unlike the PL search
// gateway. The first ItemClassificationCode is the primary CPV; any others
// become CPVSecondary.
func Map(d Document, source string) tender.Tender {
	var cpv string
	var secondary []string
	if len(d.CPV) > 0 {
		cpv = d.CPV[0]
		if len(d.CPV) > 1 {
			secondary = append([]string(nil), d.CPV[1:]...)
		}
	}

	return tender.Tender{
		Source:       source,
		SourceRef:    d.ContractFolderID,
		Title:        d.Title,
		Buyer:        tender.Buyer{Name: d.BuyerName},
		Status:       statusFromCode(d.StatusCode),
		Language:     "es",
		Country:      "ES",
		NUTS:         d.NUTS,
		CPV:          cpv,
		CPVSecondary: secondary,
		Value:        d.EstimatedValue,
		Currency:     d.Currency,
		Deadline:     d.SubmissionDeadline,
		Documents:    mapDocuments(d.Documents),
		Raw:          rawJSON(d.Raw),
	}
}

// mapDocuments turns the folder's DocumentRefs (PCAP/PCTP) into
// tender.Documents. Returns nil when the folder carries none.
func mapDocuments(refs []DocumentRef) []tender.Document {
	if len(refs) == 0 {
		return nil
	}
	docs := make([]tender.Document, len(refs))
	for i, r := range refs {
		docs[i] = tender.Document{URL: r.URI, Type: r.Type}
	}
	return docs
}

// statusFromCode maps PLACSP's ContractFolderStatusCode onto the fixed
// tender.Status enum. Verified against the CODICE
// SyndicationContractFolderStatusCode code list: PUB (publicada) and EV (en
// evaluación / bidding open) are live; ADJ (adjudicada) and RES (resuelta) are
// awarded; ANUL (anulada) is cancelled. Anything else is unknown — never
// guessed, per the domain contract.
func statusFromCode(code string) tender.Status {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "PUB", "EV":
		return tender.StatusOpen
	case "ADJ", "RES":
		return tender.StatusAwarded
	case "ANUL":
		return tender.StatusCancelled
	default:
		return tender.StatusUnknown
	}
}

// rawJSON wraps the untouched CODICE XML payload as a JSON string. The domain
// model's Raw is a json.RawMessage persisted into a jsonb column ($17::jsonb),
// so it must be valid JSON; the CODICE payload is XML, which is not. Encoding
// it as a JSON string keeps the payload intact and byte-recoverable while
// staying valid jsonb. An empty payload yields nil (no raw stored).
func rawJSON(payload []byte) json.RawMessage {
	if len(payload) == 0 {
		return nil
	}
	encoded, err := json.Marshal(string(payload))
	if err != nil {
		return nil
	}
	return encoded
}

// MapSelectionCriteria turns a document's parsed requirements into domain
// selection criteria. Ordinal is assigned here, from the order
// collectSelectionCriteria emitted — see the note on tenderingTerms for why
// that order is a fixed family sequence rather than the document's.
//
// origin is the provider name rather than a constant, so the row records which
// reading produced it and a reader can grade trust per row without this package
// deciding what trust means.
//
// LotRef is left empty: CODICE publishes the qualification block once for the
// whole contract folder, never per lot. Writing a lot ref we do not have would
// be inventing a scope the source never expressed.
func MapSelectionCriteria(d Document, source string) []tender.SelectionCriterion {
	if len(d.SelectionCriteria) == 0 {
		return nil
	}
	out := make([]tender.SelectionCriterion, len(d.SelectionCriteria))
	for i, r := range d.SelectionCriteria {
		out[i] = tender.SelectionCriterion{
			Ordinal:     i,
			Category:    r.Category,
			Type:        r.Code,
			Description: r.Description,
			Origin:      source,
			Lang:        "spa",
		}
	}
	return out
}
