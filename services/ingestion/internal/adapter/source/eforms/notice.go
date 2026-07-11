package eforms

import "encoding/json"

// Notice is the raw shape of one eForms notice as returned by TED's Search
// API, decoded from exactly the fields tedapi.Client requests. Field names
// and shapes were verified live against api.ted.europa.eu/v3/notices/search
// — see the design doc for the request/response samples this was built
// from.
type Notice struct {
	PublicationNumber            string              `json:"publication-number"`
	ProcedureIdentifier          string              `json:"procedure-identifier"`
	NoticeType                   string              `json:"notice-type"`
	ProcedureType                string              `json:"procedure-type"`
	NoticeTitle                  map[string]string   `json:"notice-title"`
	BuyerName                    map[string][]string `json:"buyer-name"`
	OrganisationIdentifierBuyer  []string            `json:"organisation-identifier-buyer"`
	OfficialLanguage             []string            `json:"official-language"`
	BuyerCountry                 []string            `json:"buyer-country"`
	ClassificationCPV            []string            `json:"classification-cpv"`
	EstimatedValueProc           string              `json:"estimated-value-proc"`
	EstimatedValueCurProc        string              `json:"estimated-value-cur-proc"`
	PublicationDate              string              `json:"publication-date"`
	IdentifierLot                []string            `json:"identifier-lot"`
	TitleLot                     map[string][]string `json:"title-lot"`
	DeadlineReceiptTenderDateLot []string            `json:"deadline-receipt-tender-date-lot"`
	DeadlineReceiptTenderTimeLot []string            `json:"deadline-receipt-tender-time-lot"`
	Links                        Links               `json:"links"`

	// Raw holds the exact bytes this Notice was decoded from, set by
	// Decode. Excluded from normal (un)marshalling — it's provenance, not
	// a field TED sends.
	Raw json.RawMessage `json:"-"`
}

// Links is the per-format, per-language document URL map TED returns.
// Verified live: keys are UPPERCASE 3-letter language codes (e.g. "RON",
// "ENG") — a different casing convention than NoticeTitle/BuyerName/
// TitleLot, which use lowercase keys. This is TED's own inconsistency, not
// a mistake in this struct.
type Links struct {
	PDF map[string]string `json:"pdf"`
}

// Decode unmarshals one raw notice object into a Notice, retaining the
// original bytes on Notice.Raw for tender.Tender.Raw.
func Decode(data []byte) (Notice, error) {
	var n Notice
	if err := json.Unmarshal(data, &n); err != nil {
		return Notice{}, err
	}
	n.Raw = append(json.RawMessage(nil), data...)
	return n, nil
}
