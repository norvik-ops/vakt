package admin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

// fetchMetadataFromURL fetches IdP metadata XML from a URL.
// Enforces a 10s timeout, a 512 KB size limit, and blocks SSRF via a custom
// DialContext that resolves, validates, and dials in one step — closing the
// DNS-rebinding TOCTOU window that a pre-flight check leaves open.
func fetchMetadataFromURL(ctx context.Context, rawURL string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return "", fmt.Errorf("metadata URL invalid: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("metadata URL must use http or https scheme")
	}

	// dialContext resolves, validates, and dials in one step.
	// All returned addresses must be public; the first validated IP is dialed
	// directly so the OS cannot re-resolve to a different address.
	dialContext := func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid dial address: %w", err)
		}
		addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("DNS lookup failed: %w", err)
		}
		if len(addrs) == 0 {
			return nil, errors.New("DNS returned no addresses")
		}
		for _, a := range addrs {
			if !isPublicIP(a.IP) {
				return nil, fmt.Errorf("metadata URL resolves to a non-public address — blocked for security")
			}
		}
		return (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(addrs[0].IP.String(), port))
	}

	client := &http.Client{
		Transport: &http.Transport{DialContext: dialContext},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("redirect to non-http(s) scheme blocked")
			}
			// IP validation for redirects is handled by dialContext on the transport.
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("metadata URL returned status %d", resp.StatusCode)
	}

	const maxBytes = 512 * 1024
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return "", fmt.Errorf("read metadata response: %w", err)
	}
	if len(body) > maxBytes {
		return "", fmt.Errorf("metadata response exceeds 512 KB limit")
	}
	return string(body), nil
}

// isPublicIP returns true only for globally routable unicast addresses.
func isPublicIP(ip net.IP) bool {
	private := []net.IPNet{
		{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(8, 32)},
		{IP: net.ParseIP("172.16.0.0"), Mask: net.CIDRMask(12, 32)},
		{IP: net.ParseIP("192.168.0.0"), Mask: net.CIDRMask(16, 32)},
		{IP: net.ParseIP("100.64.0.0"), Mask: net.CIDRMask(10, 32)},  // carrier-grade NAT
		{IP: net.ParseIP("169.254.0.0"), Mask: net.CIDRMask(16, 32)}, // link-local
		{IP: net.ParseIP("fc00::"), Mask: net.CIDRMask(7, 128)},      // IPv6 ULA
		{IP: net.ParseIP("fe80::"), Mask: net.CIDRMask(10, 128)},     // IPv6 link-local
	}
	if ip.IsLoopback() || ip.IsMulticast() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, block := range private {
		if block.Contains(ip) {
			return false
		}
	}
	return true
}
