// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vakthr

import (
	"strings"
	"testing"
)

// TestPersonioPlaceholderEmail_UniquePerID is the unit-level guard for the
// Personio-webhook 500 (R1-36c-03 Teil B): the placeholder address for an
// unknown employee must be UNIQUE per Personio id and must never be the empty
// string, because hr_employees enforces UNIQUE(org_id, email) and ” is a value,
// not a gap — two unknown departures in one org used to collide on ” and 500 the
// rest of the webhook batch.
func TestPersonioPlaceholderEmail_UniquePerID(t *testing.T) {
	a := personioPlaceholderEmail(42)
	b := personioPlaceholderEmail(99)

	if a == "" || b == "" {
		t.Fatalf("placeholder email must not be empty (that is what collides): a=%q b=%q", a, b)
	}
	if a == b {
		t.Fatalf("two distinct Personio ids must yield distinct placeholder emails: a=%q b=%q", a, b)
	}
	// Same id is deterministic — a re-delivered webhook maps to the same row.
	if got := personioPlaceholderEmail(42); got != a {
		t.Fatalf("placeholder must be deterministic for a given id: first=%q second=%q", a, got)
	}
	// The .invalid TLD (RFC 6761) guarantees the stand-in can never resolve or be mailed.
	if !strings.HasSuffix(a, "@placeholder.invalid") {
		t.Fatalf("placeholder must use a non-resolvable .invalid domain, got %q", a)
	}
	if !strings.Contains(a, "42") {
		t.Fatalf("placeholder should carry the Personio id for traceability, got %q", a)
	}
}
