package retrieve

import (
	"net/url"
	"sort"
	"strings"
)

// Address handling: the two questions this pass asks of a URL before it does
// anything with it, and the one transformation it applies to one.
//
// None of this is fetching policy — the scheme allowlist, the dial guard and
// the redirect cap all live in the adapter, where a resolved address can
// actually be refused. What is here is the part that decides what gets STORED,
// which is a property of the row rather than of the connection.

// sessionParams are query keys whose value identifies a browsing session rather
// than a document, and which therefore change on every visit.
//
// Stripping them is what makes a stored document URL stable across runs, and
// that is a correctness requirement rather than tidiness: FinishRetrieval
// deletes every tender-document row whose URL this pass did not just keep, so
// without normalisation a portal that stamps ?sid=… on its links would have
// every row deleted and re-inserted on every retrieval, and
// ingested_tender_documents would grow one dead row per file per run.
//
// The list is deliberately short and made of names that mean only this. A
// portal parameter this does not recognise survives, which is the safe way
// round: an unstripped parameter costs a churned row, where stripping a
// meaningful one would turn a working address into a broken one.
var sessionParams = map[string]bool{
	"sid":          true,
	"jsessionid":   true,
	"phpsessid":    true,
	"aspsessionid": true,
	"_csrf":        true,
	"csrf":         true,
	"token":        true,
}

// normaliseURL returns the stable form of an address: session parameters
// dropped, remaining query keys sorted, fragment removed.
//
// It is applied to what gets STORED and to what gets compared, never to what
// gets FETCHED. Those are deliberately different: a download endpoint may well
// require the very token this strips, so the request is made with the address
// the page published, and only the record of it is normalised. A tokenised URL
// that a human clicks tomorrow has an expired token either way.
//
// An address that will not parse is returned unchanged rather than dropped. It
// is not this function's job to refuse anything — the fetcher parses it again
// and refuses it there, with the sentinel that says so.
func normaliseURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return strings.TrimSpace(raw)
	}
	u.Fragment, u.RawFragment = "", ""

	if u.RawQuery != "" {
		q := u.Query()
		for key := range q {
			if sessionParams[strings.ToLower(key)] {
				q.Del(key)
			}
		}
		// url.Values.Encode already sorts by key; the explicit sort below is
		// only for the values within one repeated key, which Encode preserves
		// in insertion order and which a portal may emit in either order.
		for _, vals := range q {
			sort.Strings(vals)
		}
		u.RawQuery = q.Encode()
	}
	return u.String()
}

// hostOf returns the lowercased host of an address, or "" when it has none.
//
// It is the key the per-host circuit breaker counts against, and it is the only
// part of a URL this package ever puts in a log line or an observation: a host
// says which portal is struggling, where a full address can carry a session
// token and, on a per-document link, the buyer's own reference for a procedure.
func hostOf(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Host)
}

// tedHost and isTEDEntry recognise a documents link that points back at the EU
// register rather than at a buyer portal.
//
// It happens: a notice whose contracting authority publishes nothing of its own
// links its documents section at TED's own notice page. Following it would
// re-ingest the notice PDF that is already in this corpus as type='notice',
// double-counting the corpus against itself and spending requests on a host
// this pass has no business reading. So it is answered without a request, as a
// positive claim — there is nothing at that address beyond what we already have
// — which is why it is StatusNoFiles with a zero count rather than a NULL one.
const tedHost = "ted.europa.eu"

func isTEDEntry(raw string) bool {
	host := hostOf(raw)
	return host == tedHost || strings.HasSuffix(host, "."+tedHost)
}
