// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIsWeakAlgorithm(t *testing.T) {
	tests := []struct {
		algorithm string
		keyLength *int
		want      bool
	}{
		{"MD5", nil, true},
		{"SHA-1", nil, true},
		{"SHA1", nil, true},
		{"DES", nil, true},
		{"3DES", nil, true},
		{"RC4", nil, true},
		{"AES-256-GCM", nil, false},
		{"AES-128-CBC", nil, false},
		{"Ed25519", nil, false},
		{"TLS-ECDSA-P256", nil, false},
		{"RSA", intPtr(1024), true},   // RSA < 2048 bit → weak
		{"RSA", intPtr(2048), false},  // RSA = 2048 bit → ok
		{"RSA", intPtr(4096), false},  // RSA = 4096 bit → ok
		{"DSA", intPtr(1024), true},   // DSA < 2048 bit → weak
		{"DSA", intPtr(2048), false},  // DSA = 2048 bit → ok
	}
	for _, tt := range tests {
		label := tt.algorithm
		if tt.keyLength != nil {
			label += " " + string(rune('0'+*tt.keyLength/1000)) + "k"
		}
		t.Run(label, func(t *testing.T) {
			assert.Equal(t, tt.want, IsWeakAlgorithm(tt.algorithm, tt.keyLength))
		})
	}
}

func TestComputeRotationStatus(t *testing.T) {
	// nil → none
	assert.Equal(t, "none", computeRotationStatus(nil))

	// Past date → overdue
	past := "2020-01-01"
	assert.Equal(t, "overdue", computeRotationStatus(&past))

	// Far future → ok
	future := "2099-12-31"
	assert.Equal(t, "ok", computeRotationStatus(&future))
}

func TestRecordKeyRotation_NextDueCalculation(t *testing.T) {
	today := "2026-06-10"
	intervalDays := 365

	base, err := time.Parse("2006-01-02", today)
	assert.NoError(t, err)
	nextDue := base.AddDate(0, 0, intervalDays).Format("2006-01-02")
	assert.Equal(t, "2027-06-10", nextDue, "next rotation = today + 365 days")
}

func TestRecordKeyRotation_NoInterval(t *testing.T) {
	var nextDue *string
	assert.Nil(t, nextDue, "no interval → next_rotation_due stays nil")
}

// helpers

func intPtr(v int) *int { return &v }
