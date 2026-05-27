// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package main

import (
	"net"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

// buildXFFTrustOptions parses a comma-separated CIDR list (the value of the
// VAKT_TRUSTED_PROXIES environment variable) and returns the echo TrustOption
// slice that restricts XFF header acceptance to those ranges.
//
// Without an explicit allow-list, echo.ExtractIPFromXFFHeader() trusts every
// hop in the X-Forwarded-For chain, which allows an external client to spoof
// any IP — including admin allow-listed ones — simply by sending its own XFF
// header. The fix is to enumerate the proxy ranges we actually deploy behind
// (nginx in docker-compose, the Kubernetes node CIDR, …) and trust only those.
//
// Invalid CIDRs are logged and skipped; an entry like "172" is parsed as the
// single host 172.0.0.0/32 by net.ParseCIDR's strict mode and therefore drops
// out gracefully with a warning instead of crashing the boot.
//
// Loopback is always trusted in addition — the demo compose stack runs nginx
// and the API in the same network namespace.
func buildXFFTrustOptions(csvCIDRs string, log *zerolog.Logger) []echo.TrustOption {
	opts := []echo.TrustOption{
		echo.TrustLoopback(true),
		echo.TrustLinkLocal(false),
	}
	for _, raw := range strings.Split(csvCIDRs, ",") {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}
		_, ipnet, err := net.ParseCIDR(entry)
		if err != nil {
			if log != nil {
				log.Warn().Str("entry", entry).Err(err).Msg("VAKT_TRUSTED_PROXIES: ignoring invalid CIDR")
			}
			continue
		}
		opts = append(opts, echo.TrustIPRange(ipnet))
	}
	return opts
}
