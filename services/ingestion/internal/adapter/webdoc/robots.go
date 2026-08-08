package webdoc

// robots.txt parsing and matching.
//
// ATTRIBUTION. parseRobots, robots.allows and matchPattern below are forked
// from github.com/bernardoforcillo/tedmcp (internal/webdoc/robots.go), where
// they were written and tested against the real robots.txt files these portals
// serve. That is the expensive part: the matching rules search engines actually
// implement (longest pattern wins, Allow beats Disallow at equal length, a
// named group beats the catch-all, "*" anywhere, "$" as an end anchor) are not
// derivable from the specification text alone, and the conformance suite in
// robots_test.go is ported with them — including the portaleAppalti fixture,
// which is the live file of the second-largest Italian portal platform.
//
// It is copied rather than imported because tedmcp keeps it under internal/,
// which makes it unimportable from this module by construction — the same
// blocker the eForms parser hit and resolved the same way
// (internal/adapter/source/eforms/detail.go). Both projects have the same sole
// copyright holder, so the relicensing is his to do; the attribution stays
// because it is provenance, not a licence obligation.
//
// THREE DEVIATIONS, each a correctness change rather than a matter of taste,
// and each made where it belongs:
//
//  1. Crawl-delay is parsed and honoured (crawlDelay, and the pacer in
//     fetch.go). tedmcp's parser drops the directive on the floor: its switch
//     handles user-agent, allow and disallow and nothing else. A project whose
//     stated legal posture is robots compliance cannot read the file and then
//     ignore the one directive in it that says how fast we may go. This is the
//     most important line of the fork.
//
//  2. A robots.txt that could not be READ is not "no rules published". tedmcp
//     maps every non-200 answer to parseRobots(""), which is right for 404 and
//     410 — the file genuinely is not published — and wrong for a 503, a
//     timeout or a TLS failure, where the answer is unknown. The split lives in
//     loadRobots (fetch.go) and produces ErrRobotsUnknown, which is transient,
//     rather than silent permission.
//
//  3. The robots.txt read is bounded with the overflow treated as a hard error
//     rather than truncated (readBounded, fetch.go). A truncated robots.txt
//     parses cleanly into a SHORTER rule list, so the byte that got cut could be
//     the Disallow that governs the path we are about to fetch. Truncation here
//     fails open, which is the one direction it must never fail.

import (
	"bufio"
	"strconv"
	"strings"
	"time"
)

// robots is a parsed robots.txt: the rule groups that apply to one user agent.
type robots struct {
	groups []group
}

type group struct {
	agents []string
	rules  []rule

	// crawlDelay is the gap this group's agents must leave between requests,
	// and hasDelay distinguishes "the site published 0" from "the site
	// published nothing". Only the second lets our own floor apply — see
	// crawlDelay below.
	crawlDelay time.Duration
	hasDelay   bool
}

type rule struct {
	path  string
	allow bool
}

// parseRobots reads a robots.txt body. Unknown directives and malformed lines
// are skipped, matching how servers expect the file to be treated.
func parseRobots(body string) *robots {
	r := &robots{}
	var cur *group
	// A blank line ends a group, but consecutive User-agent lines share one.
	pendingAgents := false

	sc := bufio.NewScanner(strings.NewReader(body))
	for sc.Scan() {
		line := sc.Text()
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)

		switch key {
		case "user-agent":
			if cur == nil || !pendingAgents {
				r.groups = append(r.groups, group{})
				cur = &r.groups[len(r.groups)-1]
			}
			cur.agents = append(cur.agents, strings.ToLower(value))
			pendingAgents = true
		case "allow", "disallow":
			if cur == nil {
				continue
			}
			pendingAgents = false
			// "Disallow:" with no value is an explicit allow-all.
			if value == "" && key == "disallow" {
				continue
			}
			cur.rules = append(cur.rules, rule{path: value, allow: key == "allow"})
		case "crawl-delay":
			// DEVIATION 1. Crawl-delay is published in seconds and is
			// frequently fractional ("0.5"), so it is parsed as a float rather
			// than an integer — an integer parse would reject the fractional
			// form and silently fall back to our own floor, which is the wrong
			// way to be wrong when a site has told us its limit.
			//
			// The directive does not end an agent group the way a rule does
			// (pendingAgents is left alone): "User-agent: a / User-agent: b /
			// Crawl-delay: 5" names one group of two agents, exactly as it
			// would with a Disallow.
			if cur == nil {
				continue
			}
			secs, err := strconv.ParseFloat(value, 64)
			// Reject negatives, NaN and infinities the same way: they are not
			// a statement about pacing, and honouring one would either disable
			// the floor or block the host forever. A malformed value leaves the
			// group with no published delay, which is the standard reading of a
			// directive a server got wrong.
			if err != nil || !(secs >= 0) || secs > maxParsedCrawlDelaySeconds {
				continue
			}
			cur.crawlDelay = time.Duration(secs * float64(time.Second))
			cur.hasDelay = true
		}
	}
	return r
}

// maxParsedCrawlDelaySeconds is the largest published Crawl-delay this parser
// will represent. It is not a policy cap on politeness — the pacer honours
// whatever comes back — but a bound on absurdity: a value large enough to
// overflow a Duration, or one that reads as a typo (a day between requests),
// would turn one malformed line on one host into a goroutine parked until its
// context expires. Anything at or below this is obeyed literally; the caller's
// own per-tender deadline is what decides whether obeying it fits the run.
const maxParsedCrawlDelaySeconds = 3600

// allows reports whether userAgent may fetch path.
//
// The most specific group wins: an exact user-agent match is preferred over
// the "*" catch-all, and a site that names no applicable group allows
// everything. Within a group the longest matching pattern wins, and Allow
// beats Disallow on equal length — the behaviour search engines implement.
func (r *robots) allows(userAgent, path string) bool {
	g := r.group(userAgent)
	if g == nil {
		return true
	}

	best := rule{}
	bestLen := -1
	for _, ru := range g.rules {
		if !matchPattern(ru.path, path) {
			continue
		}
		n := len(ru.path)
		if n > bestLen || (n == bestLen && ru.allow) {
			best, bestLen = ru, n
		}
	}
	if bestLen < 0 {
		return true // no rule speaks to this path
	}
	return best.allow
}

// crawlDelay returns the gap this host asked userAgent to leave between
// requests, and whether it asked at all. A site that published no Crawl-delay
// for our group gets false, which is what lets the caller apply its own floor
// instead — see hostState.gap in fetch.go.
//
// The group selection is deliberately the same one allows uses: Crawl-delay is
// a per-group directive like any other, so a site that grants Googlebot a
// faster rate has said nothing about ours.
func (r *robots) crawlDelay(userAgent string) (time.Duration, bool) {
	g := r.group(userAgent)
	if g == nil || !g.hasDelay {
		return 0, false
	}
	return g.crawlDelay, true
}

// group picks the rule group that governs userAgent: an exact match if the file
// names one, else the "*" catch-all, else nil for a file that says nothing about
// us. Factored out of allows so that crawlDelay resolves the same group by the
// same rules — a Crawl-delay read from a group whose rules do not apply to us
// would be a number taken from someone else's permissions.
func (r *robots) group(userAgent string) *group {
	if r == nil || len(r.groups) == 0 {
		return nil
	}
	ua := productToken(userAgent)

	var exact, wildcard *group
	for i := range r.groups {
		for _, a := range r.groups[i].agents {
			if a == "*" {
				if wildcard == nil {
					wildcard = &r.groups[i]
				}
				continue
			}
			// A robots.txt agent token matches when it is a PREFIX of our
			// product token — the rule search engines implement, and the only
			// one that cannot fail open here. See productToken.
			if a != "" && strings.HasPrefix(ua, a) && exact == nil {
				exact = &r.groups[i]
			}
		}
	}

	if exact != nil {
		return exact
	}
	return wildcard
}

// productToken reduces a User-Agent header to the token robots.txt rules are
// written against: everything up to the first "/" or space, lowercased. For
// this fetcher that is "tendersbay-docs" out of
// "tendersbay-docs/1.0 (+https://tendersbay.xyz/bot)".
//
// Matching the FULL header was a fail-open bug, and the direction it failed in
// is the one this package must never fail in. group used strings.Contains over
// the whole string, so any token appearing anywhere in the header selected a
// group — including inside the contact URL. A site publishing
//
//	User-agent: bot
//	Crawl-delay: 1
//
//	User-agent: *
//	Disallow: /
//
// would have matched us on "bot" (from ".../bot"), returned that group, found
// no rule speaking to the path, and allowed the fetch — with the catch-all
// Disallow that actually governs us never consulted, because group returns the
// first named match and never falls through to the wildcard. The same shape
// fires on "docs", "https", "xyz" and every other substring a site might name.
//
// A prefix test over the product token alone cannot do that: our token is a
// fixed string this package owns, so a group is selected only when the site
// deliberately named us or a prefix of us. Callers may pass either the full
// header or a bare token — both reduce to the same value, which is what lets
// the conformance fixtures assert against either.
func productToken(userAgent string) string {
	token := strings.ToLower(strings.TrimSpace(userAgent))
	if i := strings.IndexAny(token, "/ \t"); i >= 0 {
		token = token[:i]
	}
	return token
}

// matchPattern applies robots.txt path matching: "*" stands for any run of
// characters and a trailing "$" anchors the end of the path.
func matchPattern(pattern, path string) bool {
	if pattern == "" {
		return false
	}
	anchored := strings.HasSuffix(pattern, "$")
	if anchored {
		pattern = strings.TrimSuffix(pattern, "$")
	}

	parts := strings.Split(pattern, "*")
	pos := 0
	for i, part := range parts {
		if part == "" {
			continue
		}
		var idx int
		if i == 0 {
			// The first segment must sit at the start of the path.
			if !strings.HasPrefix(path[pos:], part) {
				return false
			}
			idx = 0
		} else {
			idx = strings.Index(path[pos:], part)
			if idx < 0 {
				return false
			}
		}
		pos += idx + len(part)
	}

	if anchored {
		// With a trailing wildcard the tail is free; otherwise it must end here.
		if !strings.HasSuffix(pattern, "*") && pos != len(path) {
			return false
		}
	}
	return true
}
