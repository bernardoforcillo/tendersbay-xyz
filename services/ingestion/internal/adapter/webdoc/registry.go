package webdoc

// The listing-parser registry: which parser reads which portal.
//
// Adding a platform is one entry in NewRegistry and one file beside it, with no
// change anywhere else — the same shape, and for the same reason, as
// internal/adapter/source/registry.go. The parsers themselves are tables (see
// platform.go), so the marginal cost of the seventh Italian platform is a
// dozen lines of description and a captured fixture.

import "log/slog"

// Registry chooses a listing parser for a page and enumerates it.
//
// It is the ListingEnumerator the retrieval orchestration consumes: one page in,
// one Listing out, no I/O and no HTTP vocabulary. Everything about WHICH portal
// this is lives behind it.
type Registry struct {
	parsers  []ListingParser
	fallback ListingParser
}

// NewRegistry returns the registry with every implemented platform plus the
// generic fallback.
//
// Order matters only among the named parsers, and only when two would claim the
// same page — which none of these do today. The fallback is held separately
// rather than appended last, because it is not "the parser that happens to
// match everything": it is the one whose empty result means something
// different, and keeping it out of the list is what stops that distinction from
// being lost to a loop that treats all entries alike.
func NewRegistry() *Registry {
	return &Registry{
		parsers: []ListingParser{
			platformParser{spec: ariaSintel},
			platformParser{spec: tuttogare},
		},
		fallback: genericParser{},
	}
}

// Enumerate turns one fetched page into the files it publishes and the platform
// that read it.
//
// The first parser whose Detect claims the page wins; if none does, the
// fallback runs and the listing is reported as PlatformGeneric.
//
// # The guard, and it is the reason this method is not four lines
//
// A named platform parser is the ONLY thing in this system permitted to say
// "this listing publishes no documents" — that is a positive claim about
// absence, and the caller turns it into a status that tells a bid manager there
// is nothing to read. The fallback can never say it, because its empty result
// is a statement about the fallback (see generic.go's list of what it cannot
// catch) rather than about the page.
//
// Which puts all the weight on Detect being right. A named parser that claims a
// page it does not actually understand, finds nothing, and hands back an empty
// listing produces exactly the lie the whole package is built to prevent — and
// with fingerprints that no real capture has yet confirmed (sintel.go,
// tuttogare.go), that is not a hypothetical.
//
// So: when a named parser claims a page and enumerates nothing, the fallback is
// run over the same page before the empty result is believed. If the fallback
// finds files, the parser was wrong about the page; the fallback's files are
// returned under PlatformGeneric, because the platform column records WHICH
// ENUMERATOR produced the links and attributing them to a parser that produced
// none would hide the defect in the one query written to find it. If the
// fallback also finds nothing, the named parser's empty listing stands and the
// caller may treat it as the positive claim it is.
//
// # The partial case, which the guard deliberately does not resolve
//
// A named parser can also HALF-understand a page: claim it, return some of its
// links, and miss the rest. That produces no lie about absence — the status is
// still ok — but it understates docs_files_found, which is the one number this
// whole pass is measured on, under a platform name that reads as "a parser
// handles this portal". It is the same unverified-fingerprint risk as above,
// one notch quieter.
//
// The fallback is therefore run on this path too, and the two counts compared —
// but the named parser's result STANDS regardless. Preferring whichever set is
// larger would be the wrong rule and would quietly disable both named parsers:
// the fallback accepts any document-shaped link in the page's main content,
// while a platform parser is scoped to the attachments table precisely so a
// privacy-policy PDF in a sidebar is not counted as a tender document. Finding
// more is not the same as finding better, and this package has no capture that
// says which of the two is right on a page neither was written against.
//
// So the discrepancy is LOGGED, not acted on. It converts "silently
// understates coverage" into a line naming the platform and both counts, which
// is a thing a person can go and look at against the real page — and a real
// capture is exactly what deciding the behaviour would need. The extra cost is
// one more parse of a page already in memory, now on every named-parser hit
// rather than only on the empty ones; there is no I/O in it and there are two
// named platforms.
func (r *Registry) Enumerate(page Document) (Listing, error) {
	for _, p := range r.parsers {
		if !p.Detect(page) {
			continue
		}
		files, err := p.Parse(page)
		if err != nil {
			return Listing{}, err
		}
		fallback, err := r.fallback.Parse(page)
		if err != nil {
			return Listing{}, err
		}

		if len(files) > 0 {
			if len(fallback) > len(files) {
				slog.Warn("a platform listing parser found fewer files than the generic fallback",
					"platform", p.Name(), "url", page.URL,
					"platform_files", len(files), "fallback_files", len(fallback))
			}
			return Listing{Platform: p.Name(), Files: files}, nil
		}

		if len(fallback) > 0 {
			return Listing{Platform: PlatformGeneric, Files: fallback}, nil
		}
		return Listing{Platform: p.Name()}, nil
	}

	files, err := r.fallback.Parse(page)
	if err != nil {
		return Listing{}, err
	}
	return Listing{Platform: PlatformGeneric, Files: files}, nil
}
