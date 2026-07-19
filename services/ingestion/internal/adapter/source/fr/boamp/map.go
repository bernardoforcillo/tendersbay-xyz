// Package boamp maps a France BOAMP record (boampapi.Record) onto the shared
// tender.Tender model. It is the protocol half of the fr-boamp source: it never
// touches the network, so it can be unit-tested against captured records.
package boamp

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/fr/boampapi"
)

// Map normalizes one BOAMP record onto tender.Tender. SourceRef is idweb, the
// stable per-notice id. CPV is dug out of the nested EFORMS Donnees blob — the
// flat fields carry only BOAMP's own descripteur taxonomy, not a real CPV — and
// is left "" when the blob doesn't cleanly yield one. Value stays nil: the
// search feed's flat fields expose no clean estimate (a per-lot amount lives in
// Donnees but doesn't represent the whole-tender value), so we don't fabricate.
func Map(r boampapi.Record, source string) tender.Tender {
	cpv, secondary := extractCPV(r.Donnees)
	return tender.Tender{
		Source:       source,
		SourceRef:    r.Idweb,
		Title:        r.Objet,
		Buyer:        tender.Buyer{Name: r.NomAcheteur},
		Status:       statusFromNature(r.Nature, r.NatureCategorise),
		Country:      "FR",
		Language:     "fr",
		CPV:          cpv,
		CPVSecondary: secondary,
		Value:        nil,
		PublishedAt:  parseTime(r.DateParution),
		Deadline:     parseTime(r.DateLimiteReponse),
		Raw:          r.Raw,
	}
}

// statusFromNature maps BOAMP's avis type onto tender.Status. "attribution"
// (award notice) wins over everything; a call-for-tenders (appeloffre /
// APPEL_OFFRE / "avis de marché") is open; anything unrecognized falls to
// StatusUnknown rather than guessing.
func statusFromNature(nature, natureCategorise string) tender.Status {
	s := strings.ToLower(nature + " " + natureCategorise)
	switch {
	case strings.Contains(s, "attribution"):
		return tender.StatusAwarded
	case strings.Contains(s, "appeloffre"),
		strings.Contains(s, "appel_offre"),
		strings.Contains(s, "appel-offre"),
		strings.Contains(s, "marché"),
		strings.Contains(s, "marche"):
		return tender.StatusOpen
	default:
		return tender.StatusUnknown
	}
}

// donneesEnvelope is the outer shape of fields.donnees (itself a JSON string).
// EFORMS holds exactly one key, the notice type ("ContractNotice", …), whose
// value we don't want to hard-code — so it's a raw-message map.
type donneesEnvelope struct {
	EFORMS map[string]json.RawMessage `json:"EFORMS"`
}

type eformsNotice struct {
	ProcurementProject procurementProject `json:"cac:ProcurementProject"`
}

// procurementProject is the whole-notice project (not a per-lot project — those
// nest under cac:ProcurementProjectLot, which we deliberately ignore so the CPV
// reflects the tender, not one lot).
type procurementProject struct {
	Main       commodityClassification `json:"cac:MainCommodityClassification"`
	Additional json.RawMessage         `json:"cac:AdditionalCommodityClassification"`
}

type commodityClassification struct {
	Code itemClassificationCode `json:"cbc:ItemClassificationCode"`
}

type itemClassificationCode struct {
	ListName string `json:"@listName"`
	Text     string `json:"#text"`
}

// extractCPV digs the real CPV out of the EFORMS Donnees blob: the whole-notice
// MainCommodityClassification is primary, AdditionalCommodityClassification
// (emitted as a single object or an array) is secondary. It returns ("", nil)
// when the blob is absent or unparseable rather than fabricating a code.
func extractCPV(donnees string) (string, []string) {
	if donnees == "" {
		return "", nil
	}
	var env donneesEnvelope
	if err := json.Unmarshal([]byte(donnees), &env); err != nil {
		return "", nil
	}
	for _, rawNotice := range env.EFORMS {
		var n eformsNotice
		if err := json.Unmarshal(rawNotice, &n); err != nil {
			continue
		}
		primary := ""
		if isCPV(n.ProcurementProject.Main.Code) {
			primary = n.ProcurementProject.Main.Code.Text
		}
		secondary := extractAdditional(n.ProcurementProject.Additional)
		if primary != "" || len(secondary) > 0 {
			return primary, secondary
		}
	}
	return "", nil
}

// extractAdditional reads AdditionalCommodityClassification, which BOAMP emits
// as either one object or an array of them depending on how many there are.
func extractAdditional(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var many []commodityClassification
	if err := json.Unmarshal(raw, &many); err == nil {
		return collectCPV(many)
	}
	var one commodityClassification
	if err := json.Unmarshal(raw, &one); err == nil {
		return collectCPV([]commodityClassification{one})
	}
	return nil
}

func collectCPV(cs []commodityClassification) []string {
	var out []string
	for _, c := range cs {
		if isCPV(c.Code) {
			out = append(out, c.Code.Text)
		}
	}
	return out
}

func isCPV(c itemClassificationCode) bool {
	return c.Text != "" && (c.ListName == "" || strings.EqualFold(c.ListName, "cpv"))
}

// parseTime handles both the RFC3339 timestamps BOAMP uses for deadlines
// ("2026-08-18T08:00:00+00:00") and the bare dates it uses for publication
// ("2026-07-19"). It returns nil (never a zero time) when empty or unparseable.
func parseTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			u := t.UTC()
			return &u
		}
	}
	return nil
}
