// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package nis2wizard

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ── valueToStatus ─────────────────────────────────────────────────────────────

func TestValueToStatus_NotImplemented(t *testing.T) {
	assert.Equal(t, "not_implemented", valueToStatus(0))
	assert.Equal(t, "not_implemented", valueToStatus(1))
}

func TestValueToStatus_Partial(t *testing.T) {
	assert.Equal(t, "partial", valueToStatus(2))
}

func TestValueToStatus_Implemented(t *testing.T) {
	assert.Equal(t, "implemented", valueToStatus(3))
	assert.Equal(t, "implemented", valueToStatus(4))
}

func TestValueToStatus_AllFiveValuesDistinct(t *testing.T) {
	// Only 3 distinct outputs for 5 input values — assert the full mapping
	results := make([]string, 5)
	for i := range results {
		results[i] = valueToStatus(i)
	}
	assert.Equal(t, "not_implemented", results[0])
	assert.Equal(t, "not_implemented", results[1])
	assert.Equal(t, "partial", results[2])
	assert.Equal(t, "implemented", results[3])
	assert.Equal(t, "implemented", results[4])
}
