// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestComputeDORAAmpelStatus_ReflectsPersistedNotReconstructed pins R1-W3B-N1:
// the Ampel path must derive every traffic-light from the PERSISTED deadlines,
// not from a single anchor reconstructed as Deadline24h - 24h.
//
// The three persisted deadlines are set so that no common anchor can reproduce
// them, and specifically so that the OLD reconstruction and the persisted value
// disagree on d30:
//
//	Deadline24h = now - 1h   → red    (past)
//	Deadline72h = now + 100h → green  (far future)
//	Deadline30d = now + 1h   → yellow (<= 6h left)
//
// Old code: detectedAt = Deadline24h - 24h = now - 25h; d30 = detectedAt + 30*24h
// = now + 695h → "green". The persisted Deadline30d is now + 1h → "yellow".
// Asserting d30 == "yellow" therefore fails against the reconstructed value and
// only passes when the persisted pointer is used.
func TestComputeDORAAmpelStatus_ReflectsPersistedNotReconstructed(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)

	d24 := now.Add(-1 * time.Hour)
	d72 := now.Add(100 * time.Hour)
	d30 := now.Add(1 * time.Hour)

	inc := &Incident{
		Deadline24h: &d24,
		Deadline72h: &d72,
		Deadline30d: &d30,
	}

	status := computeDORAAmpelStatus(inc, now)

	assert.Equal(t, "red", status["h24"], "24h deadline is in the past")
	assert.Equal(t, "green", status["h72"], "72h deadline is far in the future")
	// The distinguishing assertion: reconstruction would yield "green" here.
	assert.Equal(t, "yellow", status["d30"], "30d deadline reflects persisted value (now+1h), not reconstructed (now+695h)")
}

// TestComputeDORAAmpelStatus_ReportedIsDone verifies a reported deadline wins over
// any time comparison, and that a nil persisted deadline is omitted from the map.
func TestComputeDORAAmpelStatus_ReportedIsDone(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)

	d24 := now.Add(-1 * time.Hour) // past → would be red …
	reported := now.Add(-2 * time.Hour)

	inc := &Incident{
		Deadline24h:   &d24,
		Reported24hAt: &reported,
		// Deadline72h / Deadline30d intentionally nil
	}

	status := computeDORAAmpelStatus(inc, now)

	assert.Equal(t, "done", status["h24"], "reported deadline is done regardless of clock")
	_, has72 := status["h72"]
	_, has30 := status["d30"]
	assert.False(t, has72, "nil persisted 72h deadline is omitted")
	assert.False(t, has30, "nil persisted 30d deadline is omitted")
}
