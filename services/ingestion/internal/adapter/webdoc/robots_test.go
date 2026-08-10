package webdoc

import (
	"testing"
	"time"
)

// portaleAppalti is the robots.txt served by the Maggioli "Portale Appalti"
// software that most Italian municipalities run: everything closed, with a
// narrow opening for search engines on the listing pages only.
//
// Ported verbatim from tedmcp (internal/webdoc/robots_test.go) along with the
// parser, and it is the highest-value fixture in this package. It is the live
// file of the second-largest platform in the measured corpus — nine of sixty
// sampled Italian notices, across seven distinct subdomains — and what it says
// is that a listing parser for that platform would be dead code the day it
// merged. Volume ranking applied naively would have made it the second thing
// built; this file is why it is not built at all until the operator publishes
// an agent-specific Allow, which is a business conversation rather than an
// engineering one.
const portaleAppalti = `User-agent: *
Disallow: /

User-agent: googlebot
Allow: /PortaleAppalti/it/ppgare_bandi_lista.wp*
Allow: /PortaleAppalti/it/ppgare_avvisi_lista.wp*

User-agent: bingbot
Allow: /PortaleAppalti/it/ppgare_bandi_lista.wp*
`

func TestRobotsPortaleAppalti(t *testing.T) {
	r := parseRobots(portaleAppalti)

	tests := []struct {
		name  string
		agent string
		path  string
		want  bool
	}{
		{"our agent is denied the procedure page", UserAgent, "/PortaleAppalti/it/procedure/codice/G00749", false},
		{"our agent is denied even the listing", UserAgent, "/PortaleAppalti/it/ppgare_bandi_lista.wp", false},
		{"our agent is denied the root", UserAgent, "/", false},
		{"googlebot may read the listing", "Googlebot/2.1", "/PortaleAppalti/it/ppgare_bandi_lista.wp?x=1", true},
		{"googlebot has no rule for the procedure page, so it is allowed", "Googlebot/2.1", "/PortaleAppalti/it/procedure/codice/G00749", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := r.allows(tt.agent, tt.path); got != tt.want {
				t.Fatalf("allows(%q, %q) = %v, want %v", tt.agent, tt.path, got, tt.want)
			}
		})
	}
}

func TestRobotsRules(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		agent string
		path  string
		want  bool
	}{
		{
			name: "empty robots allows everything",
			body: "", agent: "tendersbay-docs", path: "/anything", want: true,
		},
		{
			name:  "bare Disallow is an allow-all",
			body:  "User-agent: *\nDisallow:",
			agent: "tendersbay-docs", path: "/anything", want: true,
		},
		{
			name:  "longest match wins over a broader disallow",
			body:  "User-agent: *\nDisallow: /docs\nAllow: /docs/public",
			agent: "tendersbay-docs", path: "/docs/public/file.pdf", want: true,
		},
		{
			name:  "allow beats disallow at equal length",
			body:  "User-agent: *\nDisallow: /a\nAllow: /a",
			agent: "tendersbay-docs", path: "/a", want: true,
		},
		{
			name:  "wildcard in the middle of a pattern",
			body:  "User-agent: *\nDisallow: */cards/attachments/download/*",
			agent: "tendersbay-docs", path: "/x/cards/attachments/download/9", want: false,
		},
		{
			name:  "end anchor does not match a longer path",
			body:  "User-agent: *\nDisallow: /private$",
			agent: "tendersbay-docs", path: "/private/file", want: true,
		},
		{
			name:  "end anchor matches exactly",
			body:  "User-agent: *\nDisallow: /private$",
			agent: "tendersbay-docs", path: "/private", want: false,
		},
		{
			name:  "consecutive user-agent lines share one group",
			body:  "User-agent: alpha\nUser-agent: beta\nDisallow: /x",
			agent: "beta", path: "/x", want: false,
		},
		{
			name:  "a named group beats the catch-all",
			body:  "User-agent: *\nDisallow: /\n\nUser-agent: tendersbay-docs\nAllow: /open",
			agent: UserAgent, path: "/open/doc.pdf", want: true,
		},
		{
			name:  "rules of a group that does not apply are ignored",
			body:  "User-agent: googlebot\nDisallow: /",
			agent: "tendersbay-docs", path: "/anything", want: true,
		},
		{
			name:  "comments and blank lines are skipped",
			body:  "# nothing to see\n\nUser-agent: *\n  Disallow: /gare   # not for robots\n",
			agent: "tendersbay-docs", path: "/gare/123", want: false,
		},
		{
			name:  "the query string participates in matching",
			body:  "User-agent: *\nDisallow: /*?*download=",
			agent: "tendersbay-docs", path: "/gare/123?download=7", want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseRobots(tt.body).allows(tt.agent, tt.path); got != tt.want {
				t.Fatalf("allows(%q, %q) = %v, want %v", tt.agent, tt.path, got, tt.want)
			}
		})
	}
}

// TestRobotsCrawlDelay covers deviation 1 of the fork. tedmcp drops the
// directive entirely, and a project whose stated posture is robots compliance
// cannot read the file and then ignore the one line in it that says how fast we
// may go — so every shape a real file publishes it in has to survive the parse.
func TestRobotsCrawlDelay(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		agent     string
		want      time.Duration
		published bool
	}{
		{
			name:      "a whole number of seconds",
			body:      "User-agent: *\nCrawl-delay: 10\nDisallow: /admin",
			agent:     UserAgent,
			want:      10 * time.Second,
			published: true,
		},
		{
			name:      "a fractional delay, which an integer parse would have dropped",
			body:      "User-agent: *\nCrawl-delay: 0.5",
			agent:     UserAgent,
			want:      500 * time.Millisecond,
			published: true,
		},
		{
			name:      "the directive is read from OUR group, not from a faster one granted to Googlebot",
			body:      "User-agent: googlebot\nCrawl-delay: 1\n\nUser-agent: *\nCrawl-delay: 30",
			agent:     UserAgent,
			want:      30 * time.Second,
			published: true,
		},
		{
			name:      "an agent-specific group wins over the catch-all",
			body:      "User-agent: *\nCrawl-delay: 30\n\nUser-agent: tendersbay-docs\nCrawl-delay: 3",
			agent:     UserAgent,
			want:      3 * time.Second,
			published: true,
		},
		{
			name:      "the directive does not end the agent group it sits in",
			body:      "User-agent: alpha\nUser-agent: tendersbay-docs\nCrawl-delay: 4",
			agent:     UserAgent,
			want:      4 * time.Second,
			published: true,
		},
		{
			name:      "no directive published",
			body:      "User-agent: *\nDisallow: /admin",
			agent:     UserAgent,
			published: false,
		},
		{
			name:      "a malformed value publishes nothing rather than a guess",
			body:      "User-agent: *\nCrawl-delay: soon",
			agent:     UserAgent,
			published: false,
		},
		{
			name:      "a negative value is not a statement about pacing",
			body:      "User-agent: *\nCrawl-delay: -5",
			agent:     UserAgent,
			published: false,
		},
		{
			name:      "an absurd value is refused rather than parking a worker for a day",
			body:      "User-agent: *\nCrawl-delay: 999999",
			agent:     UserAgent,
			published: false,
		},
		{
			name:      "an explicit zero is published, and is distinct from silence",
			body:      "User-agent: *\nCrawl-delay: 0",
			agent:     UserAgent,
			want:      0,
			published: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseRobots(tt.body).crawlDelay(tt.agent)
			if ok != tt.published {
				t.Fatalf("crawlDelay(%q) published = %v, want %v", tt.agent, ok, tt.published)
			}
			if ok && got != tt.want {
				t.Fatalf("crawlDelay(%q) = %s, want %s", tt.agent, got, tt.want)
			}
		})
	}
}

// TestRobotsCrawlDelayIsNotPublishedByAnUnrelatedGroup pins the half of the
// group selection that is easy to get wrong by reading the file top to bottom:
// a Crawl-delay published for Googlebot and nobody else says nothing about us,
// and treating it as ours would take a number out of someone else's permissions.
func TestRobotsCrawlDelayIsNotPublishedByAnUnrelatedGroup(t *testing.T) {
	r := parseRobots("User-agent: googlebot\nCrawl-delay: 1\nAllow: /")
	if d, ok := r.crawlDelay(UserAgent); ok {
		t.Fatalf("crawlDelay = %s, published = true; want nothing published for an agent the file never names", d)
	}
}

func TestRobotsNilAllowsEverything(t *testing.T) {
	var r *robots
	if !r.allows(UserAgent, "/anything") {
		t.Fatal("a nil robots must allow everything — it is the shape a host with no file at all produces")
	}
	if _, ok := r.crawlDelay(UserAgent); ok {
		t.Fatal("a nil robots must publish no crawl delay")
	}
}

// TestRobotsGroupSelectionMatchesTheProductTokenOnly is the fail-open case, and
// it is the one direction this package must never be wrong in.
//
// Group selection used to compare robots.txt agent tokens against the WHOLE
// User-Agent header with strings.Contains, so any token appearing anywhere in
// it selected a group — including inside the contact URL our header carries.
// "bot" is a substring of ".../bot", so a site whose file names a "bot" group
// for pacing and closes everything else with a catch-all Disallow selected the
// pacing group, found no rule speaking to the path, and allowed the fetch. The
// rules that actually govern us were never consulted, because group returns the
// first named match and never falls through to the wildcard.
//
// The token is matched as a prefix of our product token instead, so a group is
// selected only when a site deliberately names us.
func TestRobotsGroupSelectionMatchesTheProductTokenOnly(t *testing.T) {
	tests := []struct {
		name string
		body string
		path string
		want bool
	}{
		{
			name: "a substring of our contact URL does not select a group",
			body: "User-agent: bot\nCrawl-delay: 1\n\nUser-agent: *\nDisallow: /\n",
			path: "/gare/123/allegato.pdf",
			want: false,
		},
		{
			name: "nor does a substring of our own product token",
			body: "User-agent: docs\nAllow: /\n\nUser-agent: *\nDisallow: /riservato\n",
			path: "/riservato/file.pdf",
			want: false,
		},
		{
			name: "nor does the host in our contact URL",
			body: "User-agent: tendersbay.xyz\nAllow: /\n\nUser-agent: *\nDisallow: /\n",
			path: "/anything",
			want: false,
		},
		{
			// The rule this replaces it with, asserted in the direction that
			// keeps named permissions working: a site that grants us a path by
			// name still grants it.
			name: "a group naming us exactly still wins over the catch-all",
			body: "User-agent: *\nDisallow: /\n\nUser-agent: tendersbay-docs\nAllow: /aperto\n",
			path: "/aperto/disciplinare.pdf",
			want: true,
		},
		{
			// robots.txt tokens match a PREFIX of the product token, which is
			// what lets an operator write "tendersbay" and mean us.
			name: "a prefix of our product token names us",
			body: "User-agent: *\nDisallow: /\n\nUser-agent: tendersbay\nAllow: /aperto\n",
			path: "/aperto/disciplinare.pdf",
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseRobots(tt.body).allows(UserAgent, tt.path); got != tt.want {
				t.Fatalf("allows(UserAgent, %q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// TestProductTokenReducesTheHeader pins the reduction both callers depend on:
// the fetcher passes the full header, the conformance fixtures above pass a
// bare token, and both have to mean the same agent.
func TestProductTokenReducesTheHeader(t *testing.T) {
	tests := []struct{ in, want string }{
		{UserAgent, "tendersbay-docs"},
		{"tendersbay-docs", "tendersbay-docs"},
		{"Googlebot/2.1", "googlebot"},
		{"  TendersBay-Docs/1.0  ", "tendersbay-docs"},
		{"simplebot (+http://example.com)", "simplebot"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := productToken(tt.in); got != tt.want {
			t.Errorf("productToken(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
