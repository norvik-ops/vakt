// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package httputil

import (
	"fmt"
	"net"
	"net/url"

	"github.com/rs/zerolog/log"
)

// privateRanges lists RFC1918/loopback/link-local/IMDS CIDR blocks that must
// not be reached by admin-configurable outbound URLs unless explicitly allowed.
var privateRanges = func() []*net.IPNet {
	cidrs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"::1/128",
		"169.254.0.0/16", // IMDS (AWS, Azure, GCP metadata)
		"fc00::/7",       // IPv6 ULA
		"0.0.0.0/8",
	}
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, n, err := net.ParseCIDR(cidr)
		if err == nil {
			nets = append(nets, n)
		}
	}
	return nets
}()

// ValidateOutboundURL validates an admin-configurable outbound URL for SSRF safety.
//
// Rules (applied regardless of allowPrivate):
//   - URL must be parseable and have a scheme (http or https)
//   - Hostname must resolve to at least one IP
//
// If allowPrivate is false (the default), the resolved IPs must not fall in
// RFC1918/loopback/link-local/IMDS ranges.
//
// When allowPrivate is true, private-IP targets are permitted but a WARN
// log entry is written so that operators can audit intentional exceptions
// (e.g. on-premises Wazuh/Prometheus inside a customer LAN).
//
// Returns a user-visible error message on failure, nil on success.
func ValidateOutboundURL(rawURL string, allowPrivate bool) error {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL must use http or https (got %q)", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL has no hostname")
	}

	ips, err := net.LookupHost(host)
	if err != nil {
		// Fail-closed: if DNS resolution fails, reject the URL.
		return fmt.Errorf("hostname %q could not be resolved: %w", host, err)
	}

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		for _, n := range privateRanges {
			if n.Contains(ip) {
				if allowPrivate {
					log.Warn().
						Str("url", rawURL).
						Str("resolved_ip", ipStr).
						Msg("outbound URL resolves to private IP — allowed by allow_private_target flag")
					return nil // caller explicitly opted in
				}
				return fmt.Errorf(
					"URL must not resolve to a private or link-local address (resolved to %s) — "+
						"set allow_private_target=true if this is an intentional internal target",
					ipStr,
				)
			}
		}
	}
	return nil
}
