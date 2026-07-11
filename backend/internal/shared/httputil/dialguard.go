// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package httputil

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// S121-F4 (F1-Inj): ValidateOutboundURL runs when an admin SAVES a target, which
// leaves a DNS-rebinding TOCTOU window: the attacker's DNS server can answer with
// a public IP for the validation and a private one (e.g. 169.254.169.254, the
// cloud metadata service) when the request is actually made. S120-12 closed that
// window for webhooks and SAML metadata by re-checking the resolved IP at DIAL
// time, but the integration collectors that talk to admin-configured hosts
// (Wazuh, Prometheus, Keycloak, GitLab, SonarQube) still used a plain
// http.Client. This is that guard, factored out so there is one implementation
// instead of a copy per call site.

// forbiddenIP reports whether an IP is in a range an outbound request must not
// reach: loopback, RFC1918/ULA private space, link-local (which covers the
// 169.254.169.254 metadata endpoint), and the unspecified address.
func forbiddenIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified()
}

// GuardedDialContext returns a DialContext that resolves the hostname and dials
// the resolved IP in one step, so the address the guard checked is exactly the
// address the connection goes to — no second lookup for an attacker to poison.
//
// When allowPrivate is true, private targets are permitted (an on-premises Wazuh
// or Prometheus inside the customer's own LAN is a legitimate, opted-in case) but
// each such dial is logged at WARN so the exception is auditable.
func GuardedDialContext(allowPrivate bool) func(ctx context.Context, network, addr string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid dial address: %w", err)
		}

		addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("DNS lookup failed for %q: %w", host, err)
		}
		if len(addrs) == 0 {
			return nil, fmt.Errorf("DNS returned no addresses for %q", host)
		}

		// Reject if ANY resolved address is forbidden — a host that resolves to
		// both a public and a private IP must not be reachable by picking the
		// public one.
		for _, a := range addrs {
			if !forbiddenIP(a.IP) {
				continue
			}
			if !allowPrivate {
				return nil, fmt.Errorf(
					"refusing to connect to %q: resolves to a private or link-local address (%s) — "+
						"enable allow_private_target if this is an intentional internal host",
					host, a.IP)
			}
			log.Warn().
				Str("host", host).
				Str("resolved_ip", a.IP.String()).
				Msg("outbound request dials a private IP — allowed by allow_private_target")
		}

		// Dial the address we just validated, not a fresh lookup.
		return dialer.DialContext(ctx, network, net.JoinHostPort(addrs[0].IP.String(), port))
	}
}

// GuardedClient returns an http.Client whose transport re-validates the resolved
// IP at dial time (see GuardedDialContext). Use it for every outbound request to
// an admin-configurable host.
func GuardedClient(timeout time.Duration, allowPrivate bool) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext:         GuardedDialContext(allowPrivate),
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
}
