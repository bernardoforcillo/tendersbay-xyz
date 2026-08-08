package webdoc

import (
	"errors"
	"net/netip"
	"strings"
	"syscall"
	"testing"
)

// TestCheckAddrRefusals is the refusal table. Every entry is an address a buyer
// portal cannot legitimately live on, and 169.254.169.254 is the one that
// matters most — the cloud metadata endpoint, and the single most valuable
// address an SSRF can reach.
//
// The IPv6 spellings of an IPv4 address are here in force, because they are the
// bypass a prefix table written only in IPv4 would miss: a resolver on a
// dual-stack host hands back ::ffff:10.0.0.5 for a name that maps to 10.0.0.5,
// and 6to4 and NAT64 embed the same address again at two other offsets.
func TestCheckAddrRefusals(t *testing.T) {
	refused := []struct {
		name string
		addr string
	}{
		{"loopback", "127.0.0.1"},
		{"loopback, elsewhere in the range", "127.0.0.53"},
		{"cloud metadata", "169.254.169.254"},
		{"link-local", "169.254.1.1"},
		{"RFC1918 ten", "10.0.0.5"},
		{"RFC1918 172.16", "172.20.10.1"},
		{"RFC1918 192.168", "192.168.1.1"},
		{"carrier-grade NAT", "100.64.0.1"},
		{"this network", "0.0.0.0"},
		{"IETF protocol assignments", "192.0.0.1"},
		{"TEST-NET-1", "192.0.2.1"},
		{"benchmarking", "198.18.0.1"},
		{"multicast", "224.0.0.1"},
		{"reserved", "240.0.0.1"},
		{"broadcast", "255.255.255.255"},
		{"IPv6 loopback", "::1"},
		{"IPv6 unspecified", "::"},
		{"IPv6 unique local", "fd00::1"},
		{"IPv6 link-local", "fe80::1"},
		{"IPv6 multicast", "ff02::1"},
		{"IPv6 documentation", "2001:db8::1"},
		{"IPv4-mapped private", "::ffff:10.0.0.5"},
		{"IPv4-mapped loopback", "::ffff:127.0.0.1"},
		{"IPv4-mapped metadata", "::ffff:169.254.169.254"},
		{"6to4 wrapping loopback", "2002:7f00:1::"},
		{"6to4 wrapping RFC1918", "2002:0a00:5::"},
		{"NAT64 wrapping loopback", "64:ff9b::7f00:1"},
	}
	for _, tt := range refused {
		t.Run(tt.name, func(t *testing.T) {
			addr := netip.MustParseAddr(tt.addr)
			err := checkAddr(addr)
			if err == nil {
				t.Fatalf("checkAddr(%s) = nil, want a refusal", tt.addr)
			}
			if !errors.Is(err, ErrAddressRefused) {
				t.Fatalf("checkAddr(%s) = %v, want ErrAddressRefused", tt.addr, err)
			}
		})
	}

	allowed := []struct {
		name string
		addr string
	}{
		{"a public IPv4", "93.184.216.34"},
		{"a public IPv4 adjacent to a reserved range", "172.32.0.1"},
		{"a public IPv4 adjacent to the CGNAT range", "100.128.0.1"},
		{"a public IPv6", "2a00:1450:4001:80f::200e"},
		{"6to4 wrapping a public address", "2002:5db8:d822::"},
	}
	for _, tt := range allowed {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkAddr(netip.MustParseAddr(tt.addr)); err != nil {
				t.Fatalf("checkAddr(%s) = %v, want nil — the guard must not refuse the public internet", tt.addr, err)
			}
		})
	}
}

// TestDialGuardRefusesNonTCPAndUnresolved covers the two shapes that are not an
// address at all. Control is only ever called with a literal address over TCP,
// so anything else reaching it means the connection was not built by the
// transport in this package, and a guard that cannot judge what it was handed
// must refuse rather than pass.
func TestDialGuardRefusesNonTCPAndUnresolved(t *testing.T) {
	tests := []struct {
		name    string
		network string
		address string
	}{
		{"a unix socket", "unix", "/var/run/docker.sock"},
		{"udp", "udp", "8.8.8.8:53"},
		{"an address with no port", "tcp4", "10.0.0.5"},
		{"a name rather than an address", "tcp4", "metadata.internal:80"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := dialGuard(tt.network, tt.address, syscall.RawConn(nil))
			if !errors.Is(err, ErrAddressRefused) {
				t.Fatalf("dialGuard(%q, %q) = %v, want ErrAddressRefused", tt.network, tt.address, err)
			}
		})
	}

	if err := dialGuard("tcp4", "93.184.216.34:443", syscall.RawConn(nil)); err != nil {
		t.Fatalf("dialGuard on a public address = %v, want nil", err)
	}
}

// TestParseFetchURLSchemes pins the allowlist. file: is the entry that earns the
// test: a documents_url of "file:///etc/passwd" costs nothing to write into a
// notice and would otherwise be read by a fetcher that only checked for "://".
func TestParseFetchURLSchemes(t *testing.T) {
	allowed := []string{
		"https://www.comune.example.it/gare/123",
		"http://appalti.example.it/PortaleAppalti/it/ppgare_bandi_lista.wp",
		"HTTPS://Appalti.Example.IT/gare",
	}
	for _, raw := range allowed {
		if _, err := parseFetchURL(raw); err != nil {
			t.Errorf("parseFetchURL(%q) = %v, want nil", raw, err)
		}
	}

	refused := []string{
		"file:///etc/passwd",
		"ftp://appalti.example.it/gare",
		"gopher://appalti.example.it/1",
		"data:text/html;base64,PGh0bWw+",
		"javascript:alert(1)",
		"https://",
		"/gare/123",
		"://not a url",
	}
	for _, raw := range refused {
		if _, err := parseFetchURL(raw); !errors.Is(err, ErrAddressRefused) {
			t.Errorf("parseFetchURL(%q) = %v, want ErrAddressRefused", raw, err)
		}
	}
}

// TestParseFetchURLErrorsDoNotQuoteTheURL guards a rule the enrichment pass
// already keeps and this package inherits: an error text carries a host, never
// a full address. Portal URLs routinely carry a session identifier in the query
// string, and an error message is the easiest place for that to end up in a log
// nobody meant to put it in.
func TestParseFetchURLErrorsDoNotQuoteTheURL(t *testing.T) {
	const raw = "ftp://appalti.example.it/gare?jsessionid=SECRETSESSIONVALUE"
	_, err := parseFetchURL(raw)
	if err == nil {
		t.Fatal("parseFetchURL: want a refusal")
	}
	if got := err.Error(); strings.Contains(got, "SECRETSESSIONVALUE") || strings.Contains(got, raw) {
		t.Fatalf("error text %q quotes the URL", got)
	}
}
