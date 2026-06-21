// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ── splitCodes ───────────────────────────────────────────────────────────────

func TestSplitCodes_Empty(t *testing.T) {
	assert.Empty(t, splitCodes(""))
}

func TestSplitCodes_Single(t *testing.T) {
	assert.Equal(t, []string{"A.5.1"}, splitCodes("A.5.1"))
}

func TestSplitCodes_Multiple(t *testing.T) {
	got := splitCodes("A.5.1, A.5.2, A.5.3")
	assert.Equal(t, []string{"A.5.1", "A.5.2", "A.5.3"}, got)
}

func TestSplitCodes_NoSpaceAfterComma(t *testing.T) {
	got := splitCodes("A.5.1,A.5.2")
	assert.Equal(t, []string{"A.5.1", "A.5.2"}, got)
}

func TestSplitCodes_TrailingCommaIgnored(t *testing.T) {
	// trailing comma produces no extra empty entry
	got := splitCodes("A.5.1, A.5.2,")
	assert.Equal(t, []string{"A.5.1", "A.5.2"}, got)
}
