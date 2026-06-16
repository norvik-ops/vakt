// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package veriniceimport

import (
	"strings"
	"testing"
)

// FuzzParseXML feeds arbitrary input to the SNCA XML parser. The parser must
// never panic on malformed / hostile input (CLAUDE.md: fuzz for parsers /
// untrusted input). Runs in CI with -fuzztime.
func FuzzParseXML(f *testing.F) {
	f.Add(sampleSNCA)
	f.Add("")
	f.Add("<syncObject extId=\"x\">")
	f.Add("<!DOCTYPE r [<!ENTITY e SYSTEM \"file:///etc/passwd\">]><r>&e;</r>")
	f.Add("<a><b><c><syncObject/></c></b></a>")
	f.Fuzz(func(t *testing.T, in string) {
		objs, err := ParseXML(strings.NewReader(in))
		if err != nil {
			return
		}
		// Invariant: a successful parse yields a well-formed preview.
		_ = BuildPreview(objs)
	})
}
