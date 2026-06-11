// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── computeDeadlines ─────────────────────────────────────────────────────────

func TestComputeDeadlines_NIS2HasNoFourHour(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	dl := computeDeadlines("nis2", base)

	assert.Nil(t, dl["4h"], "NIS2 has no 4h deadline")
	require.NotNil(t, dl["24h"])
	require.NotNil(t, dl["72h"])
	require.NotNil(t, dl["30d"])

	assert.Equal(t, base.Add(24*time.Hour), *dl["24h"])
	assert.Equal(t, base.Add(72*time.Hour), *dl["72h"])
	assert.Equal(t, base.AddDate(0, 0, 30), *dl["30d"])
}

func TestComputeDeadlines_DORAHasAllFour(t *testing.T) {
	base := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
	dl := computeDeadlines("dora", base)

	require.NotNil(t, dl["4h"])
	require.NotNil(t, dl["24h"])
	require.NotNil(t, dl["72h"])
	require.NotNil(t, dl["30d"])

	assert.Equal(t, base.Add(4*time.Hour), *dl["4h"])
	assert.Equal(t, base.Add(24*time.Hour), *dl["24h"])
	assert.Equal(t, base.Add(72*time.Hour), *dl["72h"])
	assert.Equal(t, base.AddDate(0, 0, 30), *dl["30d"])
}

func TestComputeDeadlines_UnknownTypeAllNil(t *testing.T) {
	dl := computeDeadlines("general", time.Now())
	assert.Nil(t, dl["4h"])
	assert.Nil(t, dl["24h"])
	assert.Nil(t, dl["72h"])
	assert.Nil(t, dl["30d"])
}

// ── computeDeadlineStatus ────────────────────────────────────────────────────

func TestComputeDeadlineStatus_AllNilReturnsNil(t *testing.T) {
	inc := &Incident{}
	assert.Nil(t, computeDeadlineStatus(inc))
}

func TestComputeDeadlineStatus_NIS2HasFlags(t *testing.T) {
	now := time.Now().UTC()
	d24 := now.Add(20 * time.Hour)
	d72 := now.Add(68 * time.Hour)
	d30 := now.AddDate(0, 0, 30)

	inc := &Incident{Deadline24h: &d24, Deadline72h: &d72, Deadline30d: &d30}
	status := computeDeadlineStatus(inc)

	require.NotNil(t, status)
	assert.False(t, status.Has4h)
	assert.True(t, status.Has24h)
	assert.True(t, status.Has72h)
	assert.True(t, status.Has30d)
}
