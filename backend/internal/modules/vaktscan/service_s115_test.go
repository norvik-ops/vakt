// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ── certStatus ────────────────────────────────────────────────────────────────

func TestCertStatus_Nil(t *testing.T) {
	assert.Equal(t, "unknown", certStatus(nil))
}

func TestCertStatus_Expired(t *testing.T) {
	past := time.Now().UTC().Add(-1 * time.Hour)
	assert.Equal(t, "expired", certStatus(&past))
}

func TestCertStatus_Expiring(t *testing.T) {
	soon := time.Now().UTC().Add(15 * 24 * time.Hour) // 15 days — within 30-day window
	assert.Equal(t, "expiring", certStatus(&soon))
}

func TestCertStatus_Valid(t *testing.T) {
	future := time.Now().UTC().Add(90 * 24 * time.Hour) // 90 days — valid
	assert.Equal(t, "valid", certStatus(&future))
}

func TestCertStatus_ExpiringBoundary(t *testing.T) {
	// exactly 30 days remaining is still "expiring" (30d < 30d is false, but notAfter-now < 30d is true at 29d)
	exactly29 := time.Now().UTC().Add(29 * 24 * time.Hour)
	assert.Equal(t, "expiring", certStatus(&exactly29))
	exactly31 := time.Now().UTC().Add(31 * 24 * time.Hour)
	assert.Equal(t, "valid", certStatus(&exactly31))
}

// ── taskTypeForScanner ────────────────────────────────────────────────────────

func TestTaskTypeForScanner_KnownScanners(t *testing.T) {
	assert.Equal(t, TaskScanTrivy, taskTypeForScanner("trivy"))
	assert.Equal(t, TaskScanNuclei, taskTypeForScanner("nuclei"))
	assert.Equal(t, TaskScanOpenVAS, taskTypeForScanner("openvas"))
}

func TestTaskTypeForScanner_UnknownFallback(t *testing.T) {
	assert.Equal(t, "vaktscan:scan:unknown", taskTypeForScanner("nmap"))
	assert.Equal(t, "vaktscan:scan:unknown", taskTypeForScanner(""))
	assert.Equal(t, "vaktscan:scan:unknown", taskTypeForScanner("TRIVY")) // case-sensitive
}

// ── containsPhaseDone / contains ─────────────────────────────────────────────

func TestContainsPhaseDone_Finished(t *testing.T) {
	assert.True(t, containsPhaseDone(`{"phase":"finished","msg":"ok"}`))
}

func TestContainsPhaseDone_Failed(t *testing.T) {
	assert.True(t, containsPhaseDone(`{"phase":"failed","err":"timeout"}`))
}

func TestContainsPhaseDone_Running(t *testing.T) {
	assert.False(t, containsPhaseDone(`{"phase":"running","progress":42}`))
}

func TestContainsPhaseDone_Empty(t *testing.T) {
	assert.False(t, containsPhaseDone(""))
}

func TestContains_BasicSubstring(t *testing.T) {
	assert.True(t, contains("hello world", "world"))
	assert.False(t, contains("hello world", "xyz"))
	assert.True(t, contains("abc", ""))  // empty sub always true
	assert.True(t, contains("", ""))     // both empty
	assert.False(t, contains("", "abc")) // longer sub than string
}
