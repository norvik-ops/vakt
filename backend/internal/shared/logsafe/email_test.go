// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package logsafe

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedactEmail(t *testing.T) {
	cases := map[string]string{
		// happy path
		"alice@example.org":     "***@example.org",
		"a@b.c":                 "***@b.c",
		"first.last@sub.tld.de": "***@sub.tld.de",
		// edge cases
		"":                 "",
		"no-at-sign":       "***",
		"trailing@":        "***",
		"@leading":         "***",
		"multi@at@example": "***@example",
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			assert.Equal(t, want, RedactEmail(in))
		})
	}
}

func TestRedactEmailList(t *testing.T) {
	in := []string{"alice@example.org", "", "bob@test.de"}
	got := RedactEmailList(in)
	assert.Equal(t, []string{"***@example.org", "***@test.de"}, got)

	// nil-safe
	assert.Nil(t, RedactEmailList(nil))
}

// TestRedactEmail_DoesNotLeakLocalPart guards the core property: at no point
// in the output should the local part of the original email survive. This is
// the property an auditor will check on the live log stream.
func TestRedactEmail_DoesNotLeakLocalPart(t *testing.T) {
	cases := []string{
		"alice@example.org",
		"sensitive.local-part@corp.de",
		"with+plus@gmail.com",
		"hyphen-name@x.y",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			out := RedactEmail(in)
			// Extract the local part (before @) — must not appear in output.
			for i := 0; i < len(in); i++ {
				if in[i] == '@' {
					local := in[:i]
					if len(local) > 0 {
						assert.NotContains(t, out, local, "redacted output must not contain the original local part")
					}
					break
				}
			}
		})
	}
}
