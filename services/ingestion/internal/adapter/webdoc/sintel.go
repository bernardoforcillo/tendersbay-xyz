package webdoc

import "regexp"

// ARIA/Sintel — 12 of the 60 measured Italian notices, the largest single
// platform in the sample.
//
// # THE FINGERPRINTS BELOW ARE UNVERIFIED
//
// The same warning as tuttogare.go, and it applies here with one extra caveat.
// Nobody has captured a Sintel procedure page, the tests run against synthetic
// fixtures, and this table is a hypothesis until the probe replaces it.
//
// The extra caveat: SINTEL MAY NEVER REACH THIS PARSER AT ALL. It is a
// marketplace-shaped platform, and marketplace-shaped platforms answer a
// document list with a login — 401, or the shape that matters more, a 200
// carrying a login form. Fetch already classifies both as ErrAuthRequired
// before enumeration happens, so if that is what Sintel does, the deliverable
// for these 12 notices is the honest refusal ("log in here") rather than this
// file. That is a real and cheap outcome to discover, and discovering it costs
// one probe rather than one integration.
//
// # Why it is still first
//
// Volume, and a property the other platforms do not have: ARIA runs Sintel for
// the whole of Lombardy, so this is ONE HOST serving every Lombard buyer. One
// integration covers all 12, and the fetcher's per-host serial pacing is what
// makes 12 tenders behind a single host safe to read in one run — a platform
// where the volume concentrates on one address is the case that machinery was
// built for.
var ariaSintel = platformSpec{
	name: "aria-sintel",

	// "sintel" and "ariaspa" are coined product names rather than words, which
	// is what makes them usable as markers on a buyer's own domain — the case
	// hostname cannot decide.
	markers: []string{"sintel", "ariaspa", "aria s.p.a."},

	// /wps/portal/ is the WebSphere Portal path shape ARIA serves its public
	// pages under; the form is recorded in this repository's own keyword map
	// (docs/gtm/italian-keyword-map.md), which is a real address rather than an
	// invented one. It is narrowed to the Aria namespace because /wps/portal/
	// alone is shared with every other WebSphere deployment in Italian public
	// administration.
	pathPatterns: []*regexp.Regexp{
		regexp.MustCompile(`(?i)^/wps/portal/aria/`),
		regexp.MustCompile(`(?i)/eprocdata/`),
	},

	hostSuffixes: []string{"sintel.regione.lombardia.it", "ariaspa.it"},

	container: nodeMatch{
		tags:   []string{"table", "div", "section", "ul"},
		tokens: []string{"documenti", "allegat", "documentazione", "atti"},
	},

	downloadPatterns: []*regexp.Regexp{
		regexp.MustCompile(`(?i)/eprocdata/.*(download|allegat)`),
		regexp.MustCompile(`(?i)/downloaddocument`),
	},
}
