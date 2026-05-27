// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package logsafe provides redaction helpers for structured logs.
//
// Vakt is a compliance product — logging full customer email addresses on
// INFO/WARN/ERROR levels is a GDPR-Embarrassment that prior audits flagged
// (Audit F7, 38 call sites in May 2026). Use these helpers anywhere a log
// field would otherwise contain PII.
package logsafe

import (
	"strings"
)

// RedactEmail returns a domain-anchored placeholder ("***@example.org") for
// an email address. The domain is retained because it has operational value
// (which tenant, which mail provider) without exposing the individual.
//
// Empty input returns an empty string so caller can omit the field entirely.
// Inputs that do not look like an email pass through redacted ("***") so
// non-email strings accidentally routed here don't leak.
func RedactEmail(email string) string {
	if email == "" {
		return ""
	}
	at := strings.LastIndexByte(email, '@')
	if at <= 0 || at == len(email)-1 {
		return "***"
	}
	domain := email[at+1:]
	return "***@" + domain
}

// RedactEmailList applies RedactEmail to every entry; empty strings are
// skipped, so the slice is never longer than the input. Order is preserved.
func RedactEmailList(emails []string) []string {
	if len(emails) == 0 {
		return nil
	}
	out := make([]string, 0, len(emails))
	for _, e := range emails {
		if r := RedactEmail(e); r != "" {
			out = append(out, r)
		}
	}
	return out
}
