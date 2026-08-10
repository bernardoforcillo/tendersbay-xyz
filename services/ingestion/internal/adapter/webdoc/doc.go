// Package webdoc retrieves tender documents from buyers' own procurement
// portals, within the limits those portals set for automated clients.
//
// It is the second outbound-HTTP scope that internal/adapter/document's package
// comment anticipated and refused to become: "a second scope gets a second
// fetcher, with its own robots and SSRF story". This is that fetcher, and this
// is that story. Nothing here is a general-purpose HTTP client, and the
// document package's Fetch must never be pointed at a buyer portal — it has no
// robots handling, no per-host pacing and no dial guard, by design.
//
// # The invariant
//
// AN INACCESSIBLE FILE IS NEVER MISTAKEN FOR AN ABSENT ONE.
//
// That sentence is inherited verbatim from the tedmcp project this package's
// robots parser is forked from, and it is the whole reason the package exists
// rather than a "download the PDF" helper. Tender documents are public, but the
// sites that host them are not open to bulk retrieval: most Italian procurement
// portals answer "Disallow: /" to every agent except search engines, several
// gate the procedure page behind a captcha, and the marketplace-shaped ones
// (MEPA, ARIA/Sintel) require a login. Each of those is a DIFFERENT answer to
// "why can we not read this tender's documents", each is actionable by a
// different human in a different way, and collapsing any of them into "no
// documents" is a lie that this service would then repeat to a bid manager.
//
// So every refusal leaves here as a distinct sentinel — ErrRobotsDenied,
// ErrCaptcha, ErrAuthRequired, ErrNotFound, ErrAddressRefused, ErrUnusable — and
// ErrRobotsUnknown is kept scrupulously apart from ErrRobotsDenied: a robots.txt
// we could not read is neither permission nor refusal, it is an absence of
// information, and it is transient. Merging the two would file a struggling
// server under "this portal forbids us", which is the single most misleading
// thing this package could report.
//
// # What this package owns
//
//   - robots.txt: fetched, bounded, cached per host, parsed, and OBEYED —
//     including Crawl-delay, which is the one directive a project whose stated
//     posture is robots compliance cannot read and then ignore (robots.go).
//   - Politeness: one in-flight request per host, and a minimum gap between
//     request starts on that host. These are third-party servers, many of them
//     a single VM run by a comune (fetch.go).
//   - Containment: a scheme allowlist, a dial-time refusal of every private,
//     loopback, link-local and metadata address, and a redirect cap (guard.go).
//   - Bounds: bytes per response, files per tender, bytes per tender
//     (fetch.go, budget.go).
//   - Enumeration: turning one fetched listing page into the set of files it
//     publishes, per portal PLATFORM, behind a registry with a conservative
//     generic fallback (registry.go, platform.go, generic.go). This is HTML
//     shape rather than product judgement — a parser reports what a page links
//     to and which platform it recognised, and says nothing about what an empty
//     result means.
//   - Text: extraction through the existing PDF pipeline, after which THE BYTES
//     ARE DISCARDED. There is no object storage in this design and no blob
//     column anywhere; a retrieved file exists as one Document value and, for
//     the length of one extraction, one temp file (extract.go).
//
// # What this package deliberately does not own
//
// Ranking the enumerated links, deciding what an empty listing means, and the
// queue bookkeeping that turns these sentinels into a persisted docs_status.
// Those are product decisions and belong in the core layer, per
// .claude/rules/code-organization.md: this adapter maps HTTP-level outcomes and
// page shape into domain reasons and stops there.
//
// The boundary is subtle in exactly one place, so it is stated: enumeration
// happens here, but the registry never converts an empty enumeration into a
// claim. It reports the platform and an empty set of files, and whether that
// means "this listing publishes nothing" or "no parser could read this page"
// is decided a layer up — because the answer depends on which parser produced
// it, and only the core layer owns what a status says to a human.
package webdoc
