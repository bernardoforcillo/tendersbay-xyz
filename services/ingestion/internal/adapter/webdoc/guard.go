package webdoc

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
)

// Containment for a fetcher whose addresses arrive from outside this service.
//
// State the posture plainly, because the honest version is narrower than the
// scary one. The URLs this package is pointed at come from
// ingested_tenders.documents_url, written by the enrichment pass out of TED's
// own eForms XML — a published, government-operated register. That is
// materially safer than an arbitrary-URL fetcher driven by a model, and it
// removes the prompt-injection path entirely.
//
// It does not remove the need for containment, for three reasons that are all
// mundane: a buyer types the URL into their own notice and can type anything;
// a hostname resolves to whatever DNS says today, not to whatever it said when
// the notice was published; and the pod's Cilium policy permits egress to
// `world` on 80/443 (infrastructure/kubernetes/policies/webapp-cnp.yaml), so
// there is no network-level backstop underneath this file. Depth, not paranoia.

// allowedSchemes is the scheme allowlist, checked before anything is dialled.
//
// Plain http is permitted on purpose: municipal portals still serve it, nothing
// here is authenticated, and the content is public — so a downgrade costs
// confidentiality we do not have. Everything else (file:, ftp:, data:, gopher:)
// is refused at parse time, where the refusal costs a string comparison rather
// than a connection.
var allowedSchemes = map[string]bool{"http": true, "https": true}

// deniedPrefixes are the address ranges this fetcher will not connect to.
//
// 169.254.0.0/16 is the one to keep in mind: it is the cloud metadata endpoint,
// and it is the single most valuable address an SSRF can reach. The rest are
// the private, loopback, shared-CGNAT, benchmarking, documentation, multicast
// and reserved ranges — none of which a buyer's public procurement portal can
// legitimately live on, so refusing all of them costs nothing real.
var deniedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),      // "this network"
	netip.MustParsePrefix("10.0.0.0/8"),     // RFC1918
	netip.MustParsePrefix("100.64.0.0/10"),  // RFC6598 carrier-grade NAT
	netip.MustParsePrefix("127.0.0.0/8"),    // loopback
	netip.MustParsePrefix("169.254.0.0/16"), // link-local — cloud metadata
	netip.MustParsePrefix("172.16.0.0/12"),  // RFC1918
	netip.MustParsePrefix("192.0.0.0/24"),   // IETF protocol assignments
	netip.MustParsePrefix("192.0.2.0/24"),   // TEST-NET-1
	netip.MustParsePrefix("192.168.0.0/16"), // RFC1918
	netip.MustParsePrefix("198.18.0.0/15"),  // benchmarking
	netip.MustParsePrefix("224.0.0.0/4"),    // multicast
	netip.MustParsePrefix("240.0.0.0/4"),    // reserved, incl. 255.255.255.255
	netip.MustParsePrefix("::1/128"),        // loopback
	netip.MustParsePrefix("fc00::/7"),       // unique local
	netip.MustParsePrefix("fe80::/10"),      // link-local
	netip.MustParsePrefix("ff00::/8"),       // multicast
	netip.MustParsePrefix("64:ff9b::/96"),   // NAT64 — embeds an IPv4, see below
	netip.MustParsePrefix("64:ff9b:1::/48"), // NAT64 local-use
	netip.MustParsePrefix("2001:db8::/32"),  // documentation
	netip.MustParsePrefix("100::/64"),       // discard-only
	netip.MustParsePrefix("2001:2::/48"),    // benchmarking
}

// sixToFour (2002::/16) and nat64 (64:ff9b::/96) embed an IPv4 address inside
// an IPv6 one. Like the IPv4-mapped form, each is a way to spell 127.0.0.1 that
// does not look like 127.0.0.1 to a naive prefix check.
var (
	sixToFour = netip.MustParsePrefix("2002::/16")
	nat64     = netip.MustParsePrefix("64:ff9b::/96")
)

// dialGuard refuses a connection whose RESOLVED address is not a public unicast
// one. It is installed as net.Dialer.Control, and that is the only place the
// check is real: a hostname check happens before DNS, and a URL check happens
// before redirects, so both can be walked past by a name that resolves inside
// the cluster — deliberately, or by a buyer's typo pointing at an RFC1918
// address. By the time Control runs, the address is the one the kernel is about
// to connect to, and there is no window left between the check and the use.
//
// The refusal is reported as ErrAddressRefused, which is terminal: the address
// will not become public on a later pass, so retrying it would spend the row's
// whole attempt budget on the same comparison.
func dialGuard(network, address string, _ syscall.RawConn) error {
	// Anything that is not a TCP connection to an IP is refused outright. This
	// fetcher speaks HTTP over TCP and nothing else, so a unix or udp network
	// reaching here means something other than the transport below called us.
	if network != "tcp4" && network != "tcp6" {
		return fmt.Errorf("webdoc: refusing to dial over %q: %w", network, ErrAddressRefused)
	}

	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("webdoc: refusing an unparseable dial address: %w", ErrAddressRefused)
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		// Control is called with a literal address, never a name. A name here
		// means the resolution step was skipped somehow, and an unresolved
		// name is exactly what this guard cannot judge — so refuse it.
		return fmt.Errorf("webdoc: refusing to dial an unresolved address: %w", ErrAddressRefused)
	}
	if err := checkAddr(addr); err != nil {
		return err
	}
	return nil
}

// checkAddr is dialGuard's decision, split out so the refusal table can be
// tested directly without a network.
func checkAddr(addr netip.Addr) error {
	addr = unwrapAddr(addr)

	if !addr.IsValid() || addr.IsUnspecified() {
		return fmt.Errorf("webdoc: refusing to dial an unspecified address: %w", ErrAddressRefused)
	}
	for _, p := range deniedPrefixes {
		if p.Contains(addr) {
			return fmt.Errorf("webdoc: refusing to dial %s, which is in the reserved range %s: %w",
				addr, p, ErrAddressRefused)
		}
	}
	return nil
}

// unwrapAddr reduces the ways an IPv6 address can spell an IPv4 one down to the
// IPv4 address itself, so the prefix table above only has to know about the
// canonical form.
//
// Three encodings matter, and they exist for legitimate reasons, which is
// exactly why they are usable as a bypass: ::ffff:10.0.0.5 (IPv4-mapped, what a
// dual-stack resolver hands back), 2002:0a00:0005:: (6to4), and
// 64:ff9b::10.0.0.5 (NAT64). The last is caught by the prefix table as well;
// unwrapping it too means the error names the real address instead of the
// translation prefix.
func unwrapAddr(addr netip.Addr) netip.Addr {
	if addr.Is4In6() {
		return addr.Unmap()
	}
	if sixToFour.Contains(addr) {
		b := addr.As16()
		return netip.AddrFrom4([4]byte{b[2], b[3], b[4], b[5]})
	}
	if nat64.Contains(addr) {
		b := addr.As16()
		return netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]})
	}
	return addr
}

// checkScheme is the cheap first pass: a URL this fetcher will not follow at
// all, refused before DNS and before a connection. It runs on the address we
// were given and again on every redirect hop, because "the address we asked
// for" and "the address we ended up at" are only the same thing until the
// first 302.
func checkScheme(u *url.URL) error {
	scheme := strings.ToLower(u.Scheme)
	if !allowedSchemes[scheme] {
		return fmt.Errorf("webdoc: refusing scheme %q: %w", scheme, ErrAddressRefused)
	}
	if u.Host == "" {
		return fmt.Errorf("webdoc: refusing a URL with no host: %w", ErrAddressRefused)
	}
	return nil
}

// parseFetchURL parses and vets one address before anything is dialled.
func parseFetchURL(rawURL string) (*url.URL, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		// The parse error text would quote the whole URL, including any query
		// string a portal put a session token in. It is dropped rather than
		// wrapped — this package's errors carry a host, never a full address.
		return nil, fmt.Errorf("webdoc: unparseable URL: %w", ErrAddressRefused)
	}
	if err := checkScheme(u); err != nil {
		return nil, err
	}
	return u, nil
}

// guardedDialer builds the dialer every request in this package goes through.
// control is a parameter rather than a constant so the tests in this package
// can reach an httptest server, which by definition listens on 127.0.0.1 — an
// address this guard exists to refuse. There is no exported path to it, so a
// production client is always guarded; see allowPrivateAddresses.
func guardedDialer(control func(network, address string, c syscall.RawConn) error) *net.Dialer {
	return &net.Dialer{
		Timeout:   dialTimeout,
		KeepAlive: dialKeepAlive,
		Control:   control,
	}
}
