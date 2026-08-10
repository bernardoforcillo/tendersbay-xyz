package webdoc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/document"
)

// UserAgent is how this fetcher identifies itself, and it is deliberately NOT
// the one internal/adapter/document uses ("tendersbay-ingestion/1.0"). The two
// fetchers talk to different hosts under different obligations, and robots.txt
// rules match on this exact string — so sharing one token would make a portal
// unable to grant or refuse the two independently, and would make a rule
// written for us apply to a fetcher that does not read robots.txt at all.
//
// It must never impersonate a browser or a search engine. That is not only
// etiquette: the largest single finding behind this feature is that the Maggioli
// "Portale Appalti" platform serves "Disallow: /" to everyone except Googlebot
// and Bingbot, and claiming to be Googlebot is the difference between "a
// platform we cannot serve" and fraud.
//
// The contact URL is load-bearing. A portal operator who wants to talk to us —
// to complain, or to grant an agent-specific Allow — has this address and
// nothing else, so it must serve a real page before this ships.
const UserAgent = "tendersbay-docs/1.0 (+https://tendersbay.xyz/bot)"

// Politeness and containment bounds. These are not tuning knobs: every one of
// them is a promise made to a server we do not own.
const (
	// defaultHostGap is the minimum gap between two request STARTS against one
	// host, unless that host's robots.txt asks for more.
	//
	// The enricher's 250 ms is TED's measured tolerance and does not transfer
	// here by any argument. A comune's portal instance may be a single VM
	// serving a few hundred people; 2 s is about 30 requests a minute, which is
	// below the burst one human clicking through a listing generates. It is
	// deliberately conservative and deliberately NOT measured — the honest way
	// to raise it is a portal telling us so in Crawl-delay, which is why that
	// directive is parsed at all.
	defaultHostGap = 2 * time.Second

	// maxRedirects bounds one fetch's redirect chain, mirroring the notice
	// fetcher's cap. Redirects MAY leave the host — portals legitimately
	// redirect to a file CDN — but every hop re-runs the scheme check and a
	// fresh robots check for the new host, and the dial guard applies
	// automatically because it lives at the transport.
	maxRedirects = 5

	// maxFetchAttempts is one initial attempt plus two retries — fewer than
	// document.Fetch's four, and the reduction is the point. There the budget
	// is bounded by work; here it is bounded by politeness, because every
	// attempt costs a stranger's server a request for a result that is very
	// often already decided.
	maxFetchAttempts = 3

	// maxRetryAfter is the largest server-stated delay we will sit through.
	// document.Backoff clamps a Retry-After to 30 s and sleeps; a small server
	// asking us to come back in five minutes is telling us something we should
	// obey by LEAVING, not by holding a worker and a connection slot through a
	// fifth of the run window. Above this the attempt is abandoned as transient
	// and the row is retried on a later pass, which is what the server asked
	// for.
	maxRetryAfter = 60 * time.Second

	// dialTimeout and dialKeepAlive are Go's defaults, restated because
	// installing a Control function means building the dialer by hand and
	// silently dropping them would be a change nobody intended.
	dialTimeout   = 30 * time.Second
	dialKeepAlive = 30 * time.Second

	// requestTimeout bounds one request including its body read. It is
	// generous because municipal portals are slow, and it is finite because a
	// hung connection would otherwise hold this host's single request slot for
	// the whole run.
	requestTimeout = 90 * time.Second

	// loginScanLimit bounds the body size the login-wall heuristic will judge.
	// A login wall is a small page. A 900 KiB listing that happens to contain a
	// password field somewhere in a site-wide header is not something to guess
	// about, and guessing wrong there would report auth_required for a page we
	// could have enumerated.
	loginScanLimit = 512 << 10

	// markerScanLimit bounds the human-check marker scan. Every interstitial
	// these portals serve carries its fingerprint in the head or the early
	// body; scanning a whole 20 MiB response for a captcha would be looking in
	// the one place a captcha never is.
	markerScanLimit = 64 << 10
)

// maxDocumentBytes caps a single response. It is a blast-radius bound rather
// than a working limit: real tender documents are hundreds of kilobytes, so
// 20 MiB is far above anything legitimate and still small against the job's
// pod. A response that reaches it is a loud single-file failure instead of an
// OOM that takes the whole pass down.
//
// A var, not a const, for the same reason document's retry tunables are: the
// tests shrink it so the overflow behaviour can be exercised exactly at the
// boundary without moving 20 MiB through a loopback server.
var maxDocumentBytes = 20 << 20

// maxRobotsBytes caps a robots.txt read, keeping tedmcp's bound. The overflow
// is a hard error rather than a truncation — see readBounded.
var maxRobotsBytes = 512 << 10

// Client fetches documents from buyer portals, subject to each site's
// robots.txt and to the politeness bounds above.
//
// One client is meant to be built per process and shared by every worker. The
// per-host state it carries — the robots cache, the single request slot, the
// pacer — is what makes those bounds a property of this service's egress rather
// than of one goroutine; a client per caller would multiply the outbound rate
// by the number of callers and defeat the whole file.
//
// That state is per-process and per-run, and it is not a violation of the
// stateless-service rule: it holds nothing about any tender, nothing a second
// replica would have to agree with, and nothing that must survive a restart. It
// paces one process's own egress, exactly as the notice fetcher's pacer does.
type Client struct {
	userAgent string
	hostGap   time.Duration

	// http fetches documents. Its redirect policy consults robots for each new
	// hop; see NewClient.
	http *http.Client
	// robotsHTTP fetches robots.txt itself, and its redirect policy checks the
	// scheme and the hop count only. Fetching robots.txt is exempt from
	// robots.txt — universally so — and the exemption is structural here rather
	// than merely conventional: consulting robots in order to fetch robots
	// would recurse forever on the first host that redirects /robots.txt.
	robotsHTTP *http.Client

	// backoff is document.Backoff in production, replaceable only from this
	// package's tests so a retry path can be exercised without sleeping real
	// seconds.
	backoff func(attempt int, retryAfter time.Duration) time.Duration

	// dialControl is dialGuard in production and nil only under this package's
	// own tests, which have to reach a loopback address the guard refuses. It
	// is read once per dial by the transport's closure and written only during
	// construction, before the client is handed to anyone.
	dialControl func(network, address string, c syscall.RawConn) error

	mu    sync.Mutex
	hosts map[string]*hostState
}

// Option configures a Client.
type Option func(*Client)

// WithHostGap overrides the minimum gap between request starts on one host. It
// exists for the operator, not for throughput: the only defensible reason to
// pass a smaller value is a host that has told us its own limit out of band, and
// a published Crawl-delay already raises the gap without anyone touching code.
//
// A non-positive value disables pacing outright, which is correct in a test
// against a loopback server and correct nowhere else. A published Crawl-delay
// still overrides it, because obeying the site is not a tunable.
func WithHostGap(d time.Duration) Option {
	return func(c *Client) { c.hostGap = d }
}

// allowPrivateAddresses disables the dial guard. It is unexported and has
// exactly one caller: the tests in this package, whose httptest servers all
// listen on 127.0.0.1 — an address the guard exists to refuse. There is no
// exported path to it, so a production client is always guarded.
func allowPrivateAddresses() Option {
	return func(c *Client) { c.dialControl = nil }
}

// withBackoff replaces the retry delay function. Tests only, so a 429 path can
// be exercised in milliseconds rather than in document's production seconds.
func withBackoff(fn func(attempt int, retryAfter time.Duration) time.Duration) Option {
	return func(c *Client) { c.backoff = fn }
}

// NewClient returns a Client that identifies itself honestly, obeys robots.txt,
// paces itself per host, and cannot dial a private address.
func NewClient(opts ...Option) *Client {
	c := &Client{
		userAgent:   UserAgent,
		hostGap:     defaultHostGap,
		backoff:     document.Backoff,
		dialControl: dialGuard,
		hosts:       map[string]*hostState{},
	}
	for _, opt := range opts {
		opt(c)
	}

	// The transport is this package's own, deliberately not
	// http.DefaultTransport: the dial guard is installed on it, and installing
	// it on the shared default would silently apply this package's refusals to
	// every other HTTP client in the binary — including the notice fetcher,
	// which has no business being constrained by a rule written for buyer
	// portals.
	// Proxy is nil, NOT http.ProxyFromEnvironment, and that is a security
	// decision rather than a default left unfilled. A proxy would make the dial
	// guard's stated invariant false: with one configured, DialContext is
	// handed the PROXY's address, the real target is reached over CONNECT, and
	// checkAddr never sees the address the fetch actually lands on — so
	// "there is no window left between the check and the use" (guard.go) would
	// be a comment describing a check that had been routed around.
	//
	// This is not hypothetical plumbing. The retriever pod takes envFrom
	// secretRef, which injects every key in that Secret as an environment
	// variable, so an HTTPS_PROXY entry added there for any unrelated reason
	// would silently disable SSRF containment for buyer-portal fetches — a
	// documents_url naming 169.254.169.254 would be fetched through it. An
	// egress proxy is a deployment concern this fetcher must not inherit by
	// accident; if one is ever genuinely required, it belongs here as an
	// explicit Option with the guard re-established against the real target.
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return guardedDialer(c.dialControl).DialContext(ctx, network, addr)
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          32,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}

	c.http = &http.Client{
		Timeout:   requestTimeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("webdoc: %s redirected more than %d times: %w",
					req.URL.Host, maxRedirects, ErrUnusable)
			}
			if err := checkScheme(req.URL); err != nil {
				return err
			}
			// A redirect is where a fetch quietly becomes a fetch of something
			// else, so the permission has to be re-asked for the thing we
			// actually end up at. Checking only the address we chose would
			// enforce robots against our own intent rather than against the
			// server that answered.
			return c.allowedOnRedirect(req.Context(), req.URL)
		},
	}
	c.robotsHTTP = &http.Client{
		Timeout:   requestTimeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("webdoc: %s redirected robots.txt more than %d times: %w",
					req.URL.Host, maxRedirects, ErrRobotsUnknown)
			}
			return checkScheme(req.URL)
		},
	}
	return c
}

// Fetch retrieves one URL from a buyer portal, or reports why it could not.
//
// The order of operations is the politeness contract, and it is deliberate:
// parse and vet the address, take this host's single request slot, load and
// consult its robots.txt, and only then pace and request. A path robots
// disallows therefore costs zero requests to the resource itself — the refusal
// happens before anything is asked for.
func (c *Client) Fetch(ctx context.Context, rawURL string) (Document, error) {
	u, err := parseFetchURL(rawURL)
	if err != nil {
		return Document{}, err
	}

	h := c.hostFor(u)

	// One in-flight request per host, held across the robots load and the
	// document fetch. This is load-bearing rather than decorative: the largest
	// Italian platforms are one host serving hundreds of buyers, so two tenders
	// in the same batch routinely share a host, and without this they would
	// double that host's rate exactly when the corpus is densest.
	if err := h.acquire(ctx); err != nil {
		return Document{}, err
	}
	defer h.release()

	r, err := c.robotsFor(ctx, u, h)
	if err != nil {
		return Document{}, err
	}
	if !r.allows(c.userAgent, requestPath(u)) {
		return Document{}, fmt.Errorf("webdoc: %s disallows this path for %s: %w",
			u.Host, c.userAgent, ErrRobotsDenied)
	}

	return c.get(ctx, u, h)
}

// get performs the request, retrying what document's machinery says is worth
// retrying. Every attempt re-paces, because a retry is an outbound start like
// any other — three workers whose backoffs happen to expire together would
// otherwise hit a server that is already telling us to slow down with a burst.
func (c *Client) get(ctx context.Context, u *url.URL, h *hostState) (Document, error) {
	for attempt := 0; ; attempt++ {
		last := attempt >= maxFetchAttempts-1

		if err := h.pace(ctx); err != nil {
			return Document{}, err
		}

		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if reqErr != nil {
			return Document{}, fmt.Errorf("webdoc: build request for %s: %w", u.Host, reqErr)
		}
		req.Header.Set("User-Agent", c.userAgent)
		req.Header.Set("Accept", "*/*")

		resp, doErr := c.http.Do(req)
		if doErr != nil {
			// A refusal of our own — the dial guard, or the redirect policy —
			// surfaces here wrapped in a *url.Error that quotes the address it
			// was refused for. It is re-stated rather than returned so that no
			// full URL reaches the caller's logs, and it keeps its own
			// classification: a redirect into a robots-denied path is denied,
			// not merely unreachable.
			if classified := refusalFromTransport(doErr, u.Host); classified != nil {
				return Document{}, classified
			}
			// Transport error. A GET is idempotent, so retry — unless the
			// context is done (doErr wraps ctx.Err() then) or attempts are
			// spent. Returned unclassified on purpose: per the contract in
			// outcome.go, an unnamed failure is transient.
			if last || ctx.Err() != nil {
				return Document{}, doErr
			}
			if waitErr := document.Wait(ctx, c.backoff(attempt, 0)); waitErr != nil {
				return Document{}, waitErr
			}
			continue
		}

		if resp.StatusCode >= 400 {
			retryAfter := document.ParseRetryAfter(resp.Header.Get("Retry-After"))
			retryable := document.RetryableStatus(resp.StatusCode)
			status := resp.StatusCode
			resp.Body.Close()

			if err := terminalStatus(status, u.Host); err != nil {
				return Document{}, err
			}
			// The one deviation from document's machinery, and the reason
			// ParseRetryAfter returns the server's value unclamped.
			if retryAfter > maxRetryAfter {
				return Document{}, fmt.Errorf(
					"webdoc: %s answered %d and asked us to wait %s, which is longer than we will hold a slot for",
					u.Host, status, retryAfter)
			}
			if !retryable || last {
				return Document{}, fmt.Errorf("webdoc: %s answered %d", u.Host, status)
			}
			if waitErr := document.Wait(ctx, c.backoff(attempt, retryAfter)); waitErr != nil {
				return Document{}, waitErr
			}
			continue
		}

		body, readErr := readBounded(resp.Body, maxDocumentBytes, "response from "+u.Host)
		contentType := resp.Header.Get("Content-Type")
		disposition := resp.Header.Get("Content-Disposition")
		// The address the bytes came FROM, which after a redirect is not the
		// address we asked for.
		final := resp.Request.URL
		resp.Body.Close()
		if readErr != nil {
			return Document{}, readErr
		}

		// A portal that answers 200 with a human check has not given us the
		// document, and saying so is the whole difference between "no
		// documents" and "documents exist but are gated".
		if reason := detectHumanCheck(contentType, body); reason != "" {
			return Document{}, fmt.Errorf("webdoc: %s: %s: %w", final.Host, reason, ErrCaptcha)
		}
		if looksLikeLogin(contentType, body) {
			return Document{}, fmt.Errorf("webdoc: %s served a login form in place of the resource: %w",
				final.Host, ErrAuthRequired)
		}

		return Document{
			URL:         final.String(),
			ContentType: contentType,
			Filename:    filenameFrom(disposition, final),
			Body:        body,
		}, nil
	}
}

// terminalStatus maps the status codes that are a final answer onto their
// sentinels, and returns nil for everything else. Splitting it out is what keeps
// the retry loop above readable and what makes the classification testable
// without a server.
func terminalStatus(code int, host string) error {
	switch code {
	case http.StatusNotFound, http.StatusGone:
		return fmt.Errorf("webdoc: %s answered %d: %w", host, code, ErrNotFound)
	case http.StatusUnauthorized, http.StatusForbidden:
		// 403 is grouped with 401 rather than treated as a generic refusal
		// because on these portals it overwhelmingly means "this is behind the
		// login", not "you are rate-limited". Getting that wrong in the other
		// direction would tell a bid manager the documents do not exist.
		return fmt.Errorf("webdoc: %s answered %d: %w", host, code, ErrAuthRequired)
	default:
		return nil
	}
}

// refusalFromTransport recovers the classification of a refusal this package
// made itself — in the dial guard or in the redirect policy — or returns nil
// when doErr is an ordinary transport failure and therefore transient.
//
// It is needed because both refusals are raised deep inside net/http, which
// wraps them in a *url.Error (and the dial guard's in a *net.OpError as well)
// whose message quotes the address. errors.Is sees through both wrappers, so the
// sentinel survives the trip and the text — which would carry a full URL,
// session query string and all — does not have to.
func refusalFromTransport(doErr error, host string) error {
	switch {
	case errors.Is(doErr, ErrRobotsDenied):
		return fmt.Errorf("webdoc: %s: a hop in this fetch is disallowed by robots.txt: %w", host, ErrRobotsDenied)
	case errors.Is(doErr, ErrAddressRefused):
		return fmt.Errorf("webdoc: %s: refused to dial: %w", host, ErrAddressRefused)
	case errors.Is(doErr, ErrUnusable):
		return fmt.Errorf("webdoc: %s: redirected more than %d times: %w", host, maxRedirects, ErrUnusable)
	case errors.Is(doErr, ErrRobotsUnknown):
		return fmt.Errorf("webdoc: %s: robots.txt for a redirect target could not be read: %w",
			host, ErrRobotsUnknown)
	default:
		return nil
	}
}

// allowedOnRedirect consults robots for a redirect target.
//
// It deliberately does NOT take the target host's request slot. A redirect hop
// is the continuation of one request that has already paid for a slot on the
// host it started at, and taking a second slot here would deadlock the moment
// two goroutines redirected into each other's hosts. The robots load it may
// trigger still paces itself against the target host, so the hop is not free of
// politeness — only of the mutual exclusion that cannot be held safely here.
func (c *Client) allowedOnRedirect(ctx context.Context, u *url.URL) error {
	h := c.hostFor(u)
	r, err := c.robotsFor(ctx, u, h)
	if err != nil {
		return err
	}
	if !r.allows(c.userAgent, requestPath(u)) {
		return fmt.Errorf("webdoc: %s disallows this path for %s: %w", u.Host, c.userAgent, ErrRobotsDenied)
	}
	return nil
}

// hostFor returns the per-host state for u, creating it on first sight. The key
// is scheme://host, not host: a site served over both http and https publishes
// its rules at two addresses and may well publish two different files.
func (c *Client) hostFor(u *url.URL) *hostState {
	origin := strings.ToLower(u.Scheme + "://" + u.Host)

	c.mu.Lock()
	defer c.mu.Unlock()
	h, ok := c.hosts[origin]
	if !ok {
		h = newHostState(origin, c.hostGap)
		c.hosts[origin] = h
	}
	return h
}

// robotsFor returns the host's parsed robots.txt, fetching it at most once per
// host for the lifetime of the client.
//
// Caching for the process lifetime rather than for a duration is right because
// the process IS the duration: this client lives inside one scheduled run, and
// the number of distinct hosts one run touches is bounded by its batch size, so
// the cache cannot grow without limit. A failed load is deliberately not
// cached — it is transient by contract, and caching it would turn one bad
// minute on one server into a whole run of refusals.
func (c *Client) robotsFor(ctx context.Context, u *url.URL, h *hostState) (*robots, error) {
	h.rmu.Lock()
	defer h.rmu.Unlock()
	if h.loaded {
		return h.robots, nil
	}

	r, err := c.loadRobots(ctx, h)
	if err != nil {
		return nil, err
	}

	h.robots = r
	h.loaded = true
	// A published Crawl-delay only ever raises the gap. A site asking for less
	// than our floor has not been consulted about our floor: 250 ms may be fine
	// for the CDN the directive was written against and is not fine for the
	// origin we are about to read fifteen files from.
	if d, ok := r.crawlDelay(c.userAgent); ok && d > h.gap {
		h.setGap(d)
	}
	return r, nil
}

// loadRobots fetches and parses one host's robots.txt.
//
// The classification here is deviation 2 of the fork (see robots.go): 404 and
// 410 mean the file is genuinely not published and therefore no rules apply,
// which is the standard reading. EVERYTHING ELSE — a 5xx, a 403, a transport
// error, a TLS failure, a file over the bound — means we do not know what the
// rules are, and that is ErrRobotsUnknown: transient, retried, and never
// mistaken for permission.
func (c *Client) loadRobots(ctx context.Context, h *hostState) (*robots, error) {
	if err := h.pace(ctx); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.origin+"/robots.txt", nil)
	if err != nil {
		return nil, fmt.Errorf("webdoc: build robots.txt request for %s: %w", h.origin, err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "text/plain, */*")

	resp, err := c.robotsHTTP.Do(req)
	if err != nil {
		// The context being done is the caller giving up, not the host being
		// unreadable, and conflating the two would blame a portal for our own
		// deadline.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// A dial the guard refused is not an unreadable robots.txt — it is an
		// address this fetcher will never talk to, terminal rather than
		// transient. Reporting it as ErrRobotsUnknown would spend the row's
		// whole attempt budget re-refusing the same address.
		if refused := refusalFromTransport(err, h.origin); refused != nil {
			return nil, refused
		}
		return nil, fmt.Errorf("webdoc: %s robots.txt is unreachable: %w", h.origin, ErrRobotsUnknown)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return parseRobots(""), nil
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("webdoc: %s answered %d for robots.txt: %w",
			h.origin, resp.StatusCode, ErrRobotsUnknown)
	}

	body, err := readBounded(resp.Body, maxRobotsBytes, "robots.txt from "+h.origin)
	if err != nil {
		// Including an oversized file: a robots.txt read short parses into a
		// SHORTER rule list, so the byte that got cut could be the Disallow
		// governing the path we are about to request. Truncation here fails
		// open, which is the one direction it must never fail.
		return nil, fmt.Errorf("webdoc: %s robots.txt could not be read: %w", h.origin, ErrRobotsUnknown)
	}
	return parseRobots(string(body)), nil
}

// readBounded reads r, refusing a body over the bound instead of silently
// keeping its first limit bytes.
//
// This is the whole reason the read is not a plain io.LimitReader, and the
// lesson is inherited from readBoundedNotice in internal/adapter/document:
// LimitReader truncates WITHOUT SAYING SO, and everything downstream of this
// package parses leniently. A listing page cut short parses cleanly into a
// shorter file list, so the documents past the cut simply never existed as far
// as anything downstream can tell, and the tender is persisted with a coverage
// count that is wrong and looks complete. One byte past the bound is read
// specifically to detect that there was one, and its presence is a hard error.
func readBounded(r io.Reader, limit int, what string) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, int64(limit)+1))
	if err != nil {
		return nil, fmt.Errorf("webdoc: read %s: %w", what, err)
	}
	if len(body) > limit {
		return nil, fmt.Errorf("webdoc: %s is larger than the %d byte bound: %w", what, limit, ErrUnusable)
	}
	return body, nil
}

// humanCheckMarkers are the fingerprints of the interstitials these portals
// serve instead of content. The first six are forked from tedmcp, where they
// were collected from real Italian procurement portals; the last three cover the
// commercial bot-management products that increasingly sit in front of them and
// that tedmcp predates.
var humanCheckMarkers = []struct {
	needle string
	reason string
}{
	{"frc-captcha", "the page is behind a friendly-captcha human check"},
	{"g-recaptcha", "the page is behind a reCAPTCHA human check"},
	{"h-captcha", "the page is behind an hCaptcha human check"},
	{"cf-challenge", "the page is behind a Cloudflare challenge"},
	{"request rejected", "the site's web application firewall rejected the request"},
	{"enable javascript and cookies to continue", "the page requires a browser to render"},
	{"_incapsula_resource", "the page is behind an Imperva Incapsula check"},
	{"datadome", "the page is behind a DataDome check"},
	{"awswaf", "the page is behind an AWS WAF challenge"},
}

// detectHumanCheck reports why a 200 response is not the document, or "".
func detectHumanCheck(contentType string, data []byte) string {
	if !isTextual(contentType) {
		return ""
	}
	head := strings.ToLower(string(data[:min(len(data), markerScanLimit)]))
	for _, m := range humanCheckMarkers {
		if strings.Contains(head, m.needle) {
			return m.reason
		}
	}
	return ""
}

// passwordInput and documentLink are matched against the raw bytes rather than
// a lowercased copy, so judging a page costs no allocation proportional to it.
var (
	passwordInput = regexp.MustCompile(`(?i)<input[^>]+type\s*=\s*["']?password`)
	documentLink  = regexp.MustCompile(`(?i)href\s*=\s*["'][^"']+\.(pdf|docx?|rtf|odt|xlsx?|ods|zip|7z|p7m)([?#][^"']*)?["']`)
)

// looksLikeLogin reports whether a 200 HTML page is a login wall served in place
// of the resource.
//
// This half of the auth detection is the one that matters. The marketplace-shaped
// portals do not answer 401 — they answer 200 with a login form, and a caller
// that only looked at status codes would enumerate that form, find no documents,
// and record the tender as publishing none. The test is deliberately
// conservative and requires BOTH halves: a password field present, and not one
// link to anything document-shaped. A real listing page behind a site-wide login
// header still shows its files, and this must not claim otherwise.
func looksLikeLogin(contentType string, data []byte) bool {
	if !isTextual(contentType) {
		return false
	}
	if len(data) > loginScanLimit {
		return false
	}
	return passwordInput.Match(data) && !documentLink.Match(data)
}

// hostState is one origin's share of this client's politeness: its single
// request slot, its pacer, and its cached robots.txt.
type hostState struct {
	origin string

	// sem is a one-slot semaphore rather than a mutex because a mutex cannot be
	// acquired under a context — and a worker blocked on a slow host has to be
	// releasable when the run's deadline arrives, not when that host finally
	// answers.
	sem chan struct{}

	// pmu guards next and gap. It is held only to reserve a slot, never across
	// the sleep: callers queue on the reservation rather than on each other's
	// sleep, which is the difference between N requests spaced one gap apart and
	// N callers taking turns being slow. Shape borrowed from startPacer in
	// internal/adapter/document/notice_xml.go, keyed per host instead of per
	// client because here the limit belongs to each portal, not to the upstream.
	pmu  sync.Mutex
	next time.Time
	gap  time.Duration

	// rmu guards the robots cache. It is a separate lock from sem on purpose:
	// a redirect hop consults robots for a host whose slot it does not hold, so
	// the two must be acquirable independently or the redirect path deadlocks.
	rmu    sync.Mutex
	robots *robots
	loaded bool
}

func newHostState(origin string, gap time.Duration) *hostState {
	return &hostState{
		origin: origin,
		sem:    make(chan struct{}, 1),
		gap:    gap,
	}
}

// acquire takes this host's single request slot, or returns ctx.Err().
func (h *hostState) acquire(ctx context.Context) error {
	select {
	case h.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *hostState) release() { <-h.sem }

// setGap raises the pacing interval for this host, called once its robots.txt
// has been read and found to publish a Crawl-delay.
func (h *hostState) setGap(d time.Duration) {
	h.pmu.Lock()
	defer h.pmu.Unlock()
	h.gap = d
}

// pace blocks until this caller's slot on this host arrives.
//
// A published Crawl-delay is honoured literally, with no ceiling of our own:
// obeying is the point, and a host that asks for a minute between requests is
// simply a host this run will read one or two files from. The bound on that is
// the caller's context — a per-tender deadline — rather than a cap here, so
// "we obeyed and ran out of time" stays distinguishable from "we decided the
// site's own limit was too inconvenient".
func (h *hostState) pace(ctx context.Context) error {
	h.pmu.Lock()
	if h.gap <= 0 {
		h.pmu.Unlock()
		return ctx.Err()
	}
	now := time.Now()
	if h.next.Before(now) {
		h.next = now
	}
	slot := h.next
	h.next = slot.Add(h.gap)
	h.pmu.Unlock()

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
