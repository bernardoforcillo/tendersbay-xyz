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
		Raw:          rawJSON(d.Raw),
	}
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
