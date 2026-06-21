// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ── DER.4 Cross-Mapping tests ─────────────────────────────────────────────────

func TestDER4CrossMappings_Count(t *testing.T) {
	assert.Equal(t, 12, len(der4CrossMappings), "expected 12 DER.4 cross-mapping pairs")
}

func TestDER4CrossMappings_NoLegacyISO(t *testing.T) {
	for _, m := range der4CrossMappings {
		// No A.9–A.18 legacy ISO 27001:2001 codes
		if m.tgt == "ISO27001" {
			code := m.tgtCode
			assert.NotContains(t, code, "A.9.", "found legacy ISO code %s", code)
			assert.NotContains(t, code, "A.10.", "found legacy ISO code %s", code)
			assert.NotContains(t, code, "A.11.", "found legacy ISO code %s", code)
		}
	}
}

func TestDER4CrossMappings_AllBSISide(t *testing.T) {
	for _, m := range der4CrossMappings {
		assert.Equal(t, "BSI", m.src, "src should always be BSI")
		assert.Contains(t, m.srcCode, "DER.4", "srcCode should be DER.4.x")
	}
}
