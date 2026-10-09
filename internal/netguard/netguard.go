// Package netguard is the SSRF guard for outbound HTTP requests whose
// destination is supplied by a user or taken from content: webhook targets,
// links found in pages, assets fetched by URL.
//
// A client built here resolves the destination host at dial time, refuses to
// connect when any address it resolves to is loopback, link-local, private
// or otherwise reserved, and then dials the address it checked — so a
// hostname cannot pass the check with one address and connect to another
// (DNS rebinding). Redirects are dialled through the same transport, so a
// public URL that redirects inward is refused as well.
package netguard

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// ErrBlockedAddress is returned (wrapped) by the guarded dialer when the
// destination resolves to a private or reserved address.
var ErrBlockedAddress = errors.New("URL resolves to a private or restricted address")

// blockedCIDRs are IP ranges that must never be contacted via user-supplied URLs.
var blockedCIDRs = func() []*net.IPNet {
	var blocks []*net.IPNet
	for _, cidr := range []string{
		"0.0.0.0/8",      // "this" network
		"10.0.0.0/8",     // RFC1918 private
		"100.64.0.0/10",  // CGNAT shared address space
		"127.0.0.0/8",    // IPv4 loopback
		"169.254.0.0/16", // link-local / AWS EC2 metadata
		"172.16.0.0/12",  // RFC1918 private
		"192.168.0.0/16", // RFC1918 private
		"198.18.0.0/15",  // benchmarking
		"240.0.0.0/4",    // reserved
		"::/128",         // IPv6 unspecified
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 ULA (includes fd00::/8)
		"fe80::/10",      // IPv6 link-local
	} {
		_, block, err := net.ParseCIDR(cidr)
		if err == nil {
			blocks = append(blocks, block)
		}
	}
	return blocks
}()

// IsPrivateOrReservedIP returns true if ip falls in any blocked range.
func IsPrivateOrReservedIP(ip net.IP) bool {
	for _, block := range blockedCIDRs {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// dialContext resolves the destination and checks every returned address
// before connecting to the first one.
func dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid address")
	}
	ips, err := net.DefaultResolver.LookupHost(ctx, host)
	if err != nil || len(ips) == 0 {
		return nil, fmt.Errorf("could not resolve host")
	}
	for _, rawIP := range ips {
		ip := net.ParseIP(rawIP)
		if ip == nil || IsPrivateOrReservedIP(ip) {
			return nil, ErrBlockedAddress
		}
	}
	// Connect only to the first resolved public IP
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0], port))
}

// Transport is the shared guarded transport. It never uses a proxy: a proxy
// would be dialled instead of the destination and the check would be made
// on the proxy's address.
var Transport http.RoundTripper = &http.Transport{
	DialContext:           dialContext,
	TLSHandshakeTimeout:   10 * time.Second,
	ResponseHeaderTimeout: 30 * time.Second,
	MaxIdleConns:          20,
	IdleConnTimeout:       60 * time.Second,
}

// NewClient returns an http.Client that dials through the guard.
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: Transport}
}
