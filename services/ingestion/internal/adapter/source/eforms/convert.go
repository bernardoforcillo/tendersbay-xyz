// Package eforms decodes eForms notices — the shape TED's Search API returns
// — and maps them onto the shared go-services/tender.Tender domain model.
// It does no I/O; see tedapi for the HTTP transport that produces the
// Notice values this package consumes.
package eforms

// alpha3ToAlpha2Table converts an ISO 3166-1 alpha-3 country code to
// alpha-2, covering the 27 EU member states — the only countries a TED
// buyer can be registered in. Verified against TED's buyer-country field,
// which returns alpha-3 codes (e.g. "ROU", "DEU") unlike tender.Tender's
// documented alpha-2 convention.
var alpha3ToAlpha2Table = map[string]string{
	"AUT": "AT", "BEL": "BE", "BGR": "BG", "HRV": "HR", "CYP": "CY", "CZE": "CZ",
	"DNK": "DK", "EST": "EE", "FIN": "FI", "FRA": "FR", "DEU": "DE", "GRC": "GR",
	"HUN": "HU", "IRL": "IE", "ITA": "IT", "LVA": "LV", "LTU": "LT", "LUX": "LU",
	"MLT": "MT", "NLD": "NL", "POL": "PL", "PRT": "PT", "ROU": "RO", "SVK": "SK",
	"SVN": "SI", "ESP": "ES", "SWE": "SE",
}

// alpha3ToAlpha2 returns "" for a code outside the 27 EU member states
// rather than guessing.
func alpha3ToAlpha2(code string) string {
	return alpha3ToAlpha2Table[code]
}

// lang3To1Table maps the 24 official EU language codes — verified against
// every notice's `links.pdf` map keys — to ISO 639-1, as tender.Tender.Language
// expects.
var lang3To1Table = map[string]string{
	"BUL": "bg", "SPA": "es", "CES": "cs", "DAN": "da", "DEU": "de", "EST": "et",
	"ELL": "el", "ENG": "en", "FRA": "fr", "GLE": "ga", "HRV": "hr", "ITA": "it",
	"LAV": "lv", "LIT": "lt", "HUN": "hu", "MLT": "mt", "NLD": "nl", "POL": "pl",
	"POR": "pt", "RON": "ro", "SLK": "sk", "SLV": "sl", "FIN": "fi", "SWE": "sv",
}

// lang3To1 returns "" for an unrecognized code rather than guessing.
func lang3To1(code string) string {
	return lang3To1Table[code]
}

// dedupCPV splits a notice's classification-cpv array (which may repeat —
// verified live: an 84-lot notice returned 114 CPV entries) into a primary
// code and a deduplicated list of the rest.
func dedupCPV(codes []string) (primary string, secondary []string) {
	if len(codes) == 0 {
		return "", nil
	}
	primary = codes[0]
	seen := map[string]bool{primary: true}
	for _, c := range codes[1:] {
		if !seen[c] {
			seen[c] = true
			secondary = append(secondary, c)
		}
	}
	return primary, secondary
}

// first returns the first element of vals, or "" if empty — every eForms
// array field this package reads is either empty or has its most relevant
// value first.
func first(vals []string) string {
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}
