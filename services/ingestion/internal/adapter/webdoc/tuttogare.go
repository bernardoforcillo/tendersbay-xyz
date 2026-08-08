package webdoc

import "regexp"

// TuttoGare — 7 of the 60 measured Italian notices.
//
// # THE FINGERPRINTS BELOW ARE UNVERIFIED
//
// Nobody in this repository has captured a real TuttoGare listing page. The
// tests beside this file run against SYNTHETIC fixtures written from the URL
// shapes and the table layout these portals are documented to use, which
// validates the shared machinery in platform.go and validates nothing at all
// about this table. A listing parser checked only against invented HTML is not
// a validated parser.
//
// The first real capture is what settles it, and the design names the order:
// the robots-reachability probe runs before any of this is trusted, and its
// output replaces both this table and the fixtures. Until then, treat a
// TuttoGare row in the platform column as a hypothesis that happened to match.
//
// # Why this platform and not Maggioli, which has more volume
//
// Maggioli's Portale Appalti is second by volume (9 notices, seven distinct
// subdomains, the best per-integration ratio in the sample) and it is the ONLY
// platform whose page shape this repository can actually corroborate —
// /PortaleAppalti/it/procedure/codice/<CODE> appears in a real notice in
// eforms/detail_test.go. It is still not integrated, and the reason is one file
// over: robots_test.go holds that platform's real robots.txt, and it serves
// "Disallow: /" to every agent except Googlebot and Bingbot. A Maggioli parser
// would be dead code the day it merged, because Fetch refuses the page before
// any parser is reached. Ranking platforms by volume alone would have made it
// the second thing built.
//
// TuttoGare is chosen over eSOP/i-Faber (8 notices) on a different constraint:
// eSOP renders its listing with JavaScript and there is no headless browser in
// this design, so its pages arrive here as pages with no files on them no
// matter how good the table is. TuttoGare serves a table.
var tuttogare = platformSpec{
	name: "tuttogare",

	// One marker, and the omission is deliberate. TuttoGare is a Studio Amica
	// product and so is portalegare, which the sample counts separately (5
	// notices) — so "studio amica" in a footer would claim both and file the
	// sibling platform's pages under this one's name. The platform column is a
	// backlog-prioritisation key; a value that silently merges two platforms
	// destroys exactly the number it exists to report.
	markers: []string{"tuttogare"},

	pathPatterns: []*regexp.Regexp{
		regexp.MustCompile(`(?i)/gare/id\d+`),
		regexp.MustCompile(`(?i)/gare-telematiche/`),
	},

	hostSuffixes: []string{"tuttogare.it"},

	container: nodeMatch{
		tags:   []string{"table", "div", "section", "ul"},
		tokens: []string{"documenti", "allegat", "documentazione"},
	},

	downloadPatterns: []*regexp.Regexp{
		regexp.MustCompile(`(?i)/documenti?/download`),
		regexp.MustCompile(`(?i)/download/\d+`),
	},
}
