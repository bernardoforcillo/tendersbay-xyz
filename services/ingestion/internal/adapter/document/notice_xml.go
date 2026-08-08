package document

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/core/enrich"
)

// noticeOrigin is the only origin this fetcher will talk to — see the package
// comment for why the fence exists and why it is enforced rather than merely
// written down. It is compared as scheme + host, not host alone, so a plain
// http:// downgrade or a non-default port is rejected too.
const noticeOrigin = "https://ted.europa.eu"

// noticeStartInterval is the minimum gap between two outbound notice-document
// request STARTS across every caller sharing one client. It is tedmcp's
// measured pacing against the same host, adopted unchanged: TED rate-limits
// aggressively, and the constraint belongs to the upstream rather than to our
// capacity, so it is not a number to tune up when the pod looks idle.
const noticeStartInterval = 250 * time.Millisecond

// maxNoticeXMLBytes bounds one notice document. Real eForms notices run
// 20–120 KB, so 8 MiB is roughly 65x the largest thing anyone has seen and
// still small against the job's 512Mi pod — it is a blast-radius bound, not a
// working limit, and a document that reaches it is a loud single-row failure
// rather than an OOM that takes the whole pass down.
//
// It is deliberately far below the 16–32 MiB caps tedmcp uses. A cap that no
// realistic document could ever approach is not protection; it only turns an
// unbounded read into a very large one.
//
// A var, not a const, for the same reason the retry tunables above are: tests
// shrink it so the truncation behaviour below can be exercised exactly at the
// boundary without moving 8 MiB through a loopback server.
var maxNoticeXMLBytes = 8 << 20

// maxNoticeRedirects bounds how many hops one notice fetch may follow. TED does
// redirect within its own site, so zero would be wrong; ten (Go's default) is
// more than any legitimate answer needs and is a cheap loop to make someone
// else pay for.
const maxNoticeRedirects = 5

// NoticeXMLClient fetches eForms notice documents from TED and implements the
// enrich.NoticeFetcher port.
//
// One client is meant to be built per process and shared by every worker: the
// pacer it carries is what makes the 250 ms interval a property of this
// service's egress rather than of a single goroutine, and a client per caller
// would multiply the outbound rate by the number of callers.
type NoticeXMLClient struct {
	// origin is the fence from the package comment, held as a field only so
	// tests can point the client at a loopback server. Nothing outside this
	// package can change it; the exported constructor always installs
	// noticeOrigin.
	origin string
	pacer  *startPacer

	// http is this client's own, deliberately NOT the package-level one Fetch
	// uses — see NewNoticeXMLClient for why the fence needs a redirect policy
	// that Fetch must not have.
	http *http.Client
}

// NewNoticeXMLClient returns a client fenced to TED and paced at the interval
// that host tolerates.
func NewNoticeXMLClient() *NoticeXMLClient {
	c := &NoticeXMLClient{
		origin: noticeOrigin,
		pacer:  &startPacer{interval: noticeStartInterval},
	}

	// The fence has to hold across redirects, and this is the only place it
	// can. Go's default policy follows up to ten hops to ANY host, so checking
	// the address we chose and then reading whatever the last hop served would
	// enforce the allowlist against our own intent rather than against the
	// server we actually talked to — one 302 out of ted.europa.eu and the
	// arbitrary-URL fetcher the package comment forbids exists after all.
	//
	// This is exactly why the client is per-NoticeXMLClient rather than the
	// package-level httpClient: Fetch downloads buyer attachments from wherever
	// the notice said they live and MUST follow arbitrary redirects, so the two
	// need opposite policies. They still share http.DefaultTransport, and so
	// the same connection pool, because neither sets one.
	c.http = &http.Client{
		Timeout: httpClient.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxNoticeRedirects {
				return fmt.Errorf("document: notice document redirected more than %d times: %w",
					maxNoticeRedirects, enrich.ErrNoticeUnusable)
			}
			return c.checkOrigin(req.URL.String())
		},
	}
	return c
}

// FetchXML retrieves one notice document, honouring the enrich.NoticeFetcher
// error contract: enrich.ErrNoticeAbsent when the register says the document
// does not exist, enrich.ErrNoticeUnusable when a document arrives that this
// pass can never use, and any other error for a failure that might not repeat.
//
// Transient failures (429, 502/503/504, transport errors) are retried with
// exponential backoff, honouring a server-sent Retry-After — the same
// machinery Fetch uses, reused rather than restated so that a change to what
// TED considers polite is made in one place. A 404 is not among them: it is
// the register's final answer that a notice predates eForms or is of a type it
// publishes no XML for, and retrying it would turn a legacy-heavy corpus into
// five times the traffic for the same result.
//
// Errors returned from here are logged verbatim by the orchestrator, so none
// of them carry the URL: a notice document address is not itself sensitive,
// but the enrichment pass has a standing rule that no free text and no URL
// leaves it, and an error message is the easiest place for that rule to erode.
func (c *NoticeXMLClient) FetchXML(ctx context.Context, rawURL string) ([]byte, error) {
	if err := c.checkOrigin(rawURL); err != nil {
		return nil, err
	}

	for attempt := 0; ; attempt++ {
		last := attempt >= maxAttempts-1

		// Paced here rather than once per call, so that a retry costs a slot
		// like any other request. A retry is an outbound start too, and four
		// workers whose backoffs happen to expire together would otherwise hit
		// a service that is already throttling us with a burst.
		if err := c.pacer.wait(ctx); err != nil {
			return nil, err
		}

		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if reqErr != nil {
			return nil, fmt.Errorf("document: build notice document request: %w", reqErr)
		}
		req.Header.Set("User-Agent", userAgent)

		resp, doErr := c.http.Do(req)
		if doErr != nil {
			// A redirect the fence refused surfaces here wrapped in a
			// *url.Error that quotes the address it was refused for. It is
			// re-stated rather than returned so that no URL reaches the
			// orchestrator's logs, and it is terminal for the row: a redirect
			// leaving the allowlist will still be leaving it next pass, so
			// retrying it would spend five attempts on the same comparison.
			if errors.Is(doErr, enrich.ErrNoticeUnusable) {
				return nil, fmt.Errorf("document: notice document redirected outside the allowlist: %w",
					enrich.ErrNoticeUnusable)
			}
			// Transport error. A GET is idempotent, so retry — unless the
			// context is done (doErr wraps ctx.Err() then) or attempts are
			// spent. Returned unwrapped-into-a-sentinel on purpose: an
			// unclassified failure is retryable by default, which costs a
			// bounded number of attempts, where guessing "terminal" would park
			// rows that were only briefly unreachable.
			if last || ctx.Err() != nil {
				return nil, doErr
			}
			if waitErr := wait(ctx, backoff(attempt, 0)); waitErr != nil {
				return nil, waitErr
			}
			continue
		}

		if resp.StatusCode == http.StatusNotFound {
			resp.Body.Close()
			return nil, fmt.Errorf("document: notice document not published: %w", enrich.ErrNoticeAbsent)
		}

		if resp.StatusCode >= 400 {
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			retryable := retryableStatus(resp.StatusCode)
			resp.Body.Close()
			if !retryable || last {
				return nil, fmt.Errorf("document: notice document: unexpected status %d", resp.StatusCode)
			}
			if waitErr := wait(ctx, backoff(attempt, retryAfter)); waitErr != nil {
				return nil, waitErr
			}
			continue
		}

		body, readErr := readBoundedNotice(resp.Body)
		resp.Body.Close()
		return body, readErr
	}
}

// checkOrigin is the scope fence. It runs before anything is dialled, so a URL
// outside the allowlist costs a string comparison rather than a request — and
// again from CheckRedirect for every hop, because "the address we asked for"
// and "the server that answered" are only the same thing until the first 302.
//
// The failure is reported as enrich.ErrNoticeUnusable, which makes it terminal
// for the row. That is the right shape even though nothing was fetched: the
// address will not become fetchable on a later pass, so retrying it five times
// would spend the row's whole attempt budget on the same comparison. It should
// be a number that stays at zero in production — the queue's URLs come from
// TED's own links field — so any occurrence is a signal that something started
// writing addresses into xml_url from somewhere else.
func (c *NoticeXMLClient) checkOrigin(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		// The parse error text would quote the URL; it is dropped rather than
		// wrapped, per the no-URLs-in-logs rule above.
		return fmt.Errorf("document: notice document URL is unparseable: %w", enrich.ErrNoticeUnusable)
	}
	// Origin, not host: scheme is part of what is being allowed, so an http://
	// downgrade of the same host does not pass. An empty Host (a relative or
	// opaque URL) fails the comparison for free.
	if origin := strings.ToLower(u.Scheme + "://" + u.Host); origin != c.origin {
		return fmt.Errorf("document: notice document origin %q is outside the allowlist: %w",
			origin, enrich.ErrNoticeUnusable)
	}
	return nil
}

// readBoundedNotice reads a notice document, refusing one that is over the
// bound instead of silently keeping its first maxNoticeXMLBytes bytes.
//
// This is the whole reason the read is not a plain io.LimitReader. LimitReader
// truncates without saying so, and the notice parser runs with Strict = false,
// so a document cut short mid-lot decodes cleanly into a smaller notice: the
// missing lots and their criteria simply never existed as far as anything
// downstream can tell, and the row is persisted as a successful, complete
// parse. A silent undercount of exactly the numbers this pass exists to
// measure is far worse than a loud refusal, so one byte past the bound is read
// specifically to detect that there was one, and its presence is a hard error.
//
// tedmcp does not do this — it hands a bare io.LimitReader to its decoder.
// That line is not safe to copy here uncritically: tedmcp is an interactive
// tool where a human sees the result immediately, while this writes to a
// database nobody re-reads against the source.
func readBoundedNotice(r io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, int64(maxNoticeXMLBytes)+1))
	if err != nil {
		return nil, fmt.Errorf("document: read notice document: %w", err)
	}
	if len(body) > maxNoticeXMLBytes {
		return nil, fmt.Errorf("document: notice document is larger than the %d byte bound: %w",
			maxNoticeXMLBytes, enrich.ErrNoticeUnusable)
	}
	return body, nil
}

// startPacer serialises the start of every outbound request so that no more
// than one begins per interval, however many goroutines are calling. It
// reserves a slot under the lock and sleeps outside it, so callers queue on the
// reservation rather than on each other's sleep — the difference between N
// requests spaced one interval apart and N callers taking turns being slow.
//
// The enrichment orchestrator paces its own fetch dispatch as well, and that is
// not redundant: this pacer is what makes the client correct on its own terms.
// The upstream's rate limit is a fact about TED, so it belongs to the thing
// that knows TED exists, not to whichever caller happens to be driving it —
// and unlike the orchestrator's, this one also covers the retries above, which
// are requests the caller never sees.
//
// Per-process, per-run state, and not a violation of the stateless-service
// rule: it holds nothing about any tender, nothing a second replica would have
// to agree with, and nothing that must survive a restart. It paces one
// process's own egress.
type startPacer struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
}

// wait blocks until this caller's paced slot arrives, returning ctx.Err() if
// the context is done first. A non-positive interval disables pacing.
func (p *startPacer) wait(ctx context.Context) error {
	if p.interval <= 0 {
		return ctx.Err()
	}

	p.mu.Lock()
	now := time.Now()
	if p.next.Before(now) {
		p.next = now
	}
	slot := p.next
	p.next = slot.Add(p.interval)
	p.mu.Unlock()

	d := time.Until(slot)
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
