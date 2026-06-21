// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ── Truncate ─────────────────────────────────────────────────────────────────

func TestTruncate_ShortString(t *testing.T) {
	assert.Equal(t, "hello", Truncate("hello", 10))
}

func TestTruncate_ExactMax(t *testing.T) {
	assert.Equal(t, "hello", Truncate("hello", 5))
}

func TestTruncate_Overflow(t *testing.T) {
	result := Truncate("hello world", 8)
	assert.Len(t, []rune(result), 8)
	assert.True(t, strings.HasSuffix(result, "…"))
}

func TestTruncate_EmptyString(t *testing.T) {
	assert.Equal(t, "", Truncate("", 5))
}

// ── statusLabel ──────────────────────────────────────────────────────────────

func TestStatusLabel_Implemented(t *testing.T) {
	assert.Equal(t, "Implementiert", statusLabel("implemented"))
}

func TestStatusLabel_Partial(t *testing.T) {
	assert.Equal(t, "Teilweise", statusLabel("partial"))
}

func TestStatusLabel_Planned(t *testing.T) {
	assert.Equal(t, "Geplant", statusLabel("planned"))
}

func TestStatusLabel_Unknown(t *testing.T) {
	assert.Equal(t, "Nicht begonnen", statusLabel("not_started"))
	assert.Equal(t, "Nicht begonnen", statusLabel(""))
	assert.Equal(t, "Nicht begonnen", statusLabel("IMPLEMENTED"))
}
