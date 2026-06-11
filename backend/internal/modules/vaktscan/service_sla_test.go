// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ── slaDaysForSeverity ───────────────────────────────────────────────────────

func TestSlaDaysForSeverity_AllSeverities(t *testing.T) {
	cfg := &SLAConfig{
		CriticalDays: 7,
		HighDays:     30,
		MediumDays:   60,
		LowDays:      90,
	}

	assert.Equal(t, 7, slaDaysForSeverity(cfg, "critical"))
	assert.Equal(t, 30, slaDaysForSeverity(cfg, "high"))
	assert.Equal(t, 60, slaDaysForSeverity(cfg, "medium"))
	assert.Equal(t, 90, slaDaysForSeverity(cfg, "low"))
}

func TestSlaDaysForSeverity_UnknownReturnsNinetyDays(t *testing.T) {
	cfg := &SLAConfig{CriticalDays: 7, HighDays: 30, MediumDays: 60, LowDays: 90}
	assert.Equal(t, 90, slaDaysForSeverity(cfg, "informational"))
	assert.Equal(t, 90, slaDaysForSeverity(cfg, ""))
	assert.Equal(t, 90, slaDaysForSeverity(cfg, "CRITICAL"))
}

func TestSlaDaysForSeverity_RespectsConfigValues(t *testing.T) {
	// Ensure the function reads from config, not from hardcoded defaults
	cfg := &SLAConfig{CriticalDays: 3, HighDays: 14, MediumDays: 45, LowDays: 120}
	assert.Equal(t, 3, slaDaysForSeverity(cfg, "critical"))
	assert.Equal(t, 14, slaDaysForSeverity(cfg, "high"))
}
