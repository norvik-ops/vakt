// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"testing"
	"time"
)

func TestBackupStaleness(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	ptr := func(d time.Duration) *time.Time { t := now.Add(-d); return &t }

	cases := []struct {
		name       string
		last       *time.Time
		freq       string
		lastStatus string
		want       string
	}{
		{"never run is overdue", nil, "daily", "unknown", "overdue"},
		{"failed last run is overdue", ptr(time.Hour), "daily", "failed", "overdue"},
		{"daily within interval on_track", ptr(2 * time.Hour), "daily", "success", "on_track"},
		{"daily just over interval at_risk", ptr(30 * time.Hour), "daily", "success", "at_risk"},
		{"daily over 2x overdue", ptr(72 * time.Hour), "daily", "success", "overdue"},
		{"weekly within interval on_track", ptr(3 * 24 * time.Hour), "weekly", "success", "on_track"},
		{"weekly over 2x overdue", ptr(20 * 24 * time.Hour), "weekly", "success", "overdue"},
		{"hourly fresh on_track", ptr(30 * time.Minute), "hourly", "success", "on_track"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := backupStaleness(tc.last, tc.freq, tc.lastStatus, now)
			if got != tc.want {
				t.Errorf("backupStaleness=%q want %q", got, tc.want)
			}
		})
	}
}

func TestRestoreStaleness(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	dayPtr := func(days int) *time.Time { t := now.Add(-time.Duration(days) * 24 * time.Hour); return &t }

	cases := []struct {
		name       string
		last       *time.Time
		maxAgeDays int
		want       string
	}{
		{"no test is overdue", nil, 365, "overdue"},
		{"recent test on_track", dayPtr(30), 365, "on_track"},
		{"past 80% at_risk", dayPtr(320), 365, "at_risk"},
		{"older than max overdue", dayPtr(400), 365, "overdue"},
		{"zero max falls back to 365 default", dayPtr(30), 0, "on_track"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := restoreStaleness(tc.last, tc.maxAgeDays, now)
			if got != tc.want {
				t.Errorf("restoreStaleness=%q want %q", got, tc.want)
			}
		})
	}
}

func TestFrequencyInterval(t *testing.T) {
	cases := map[string]time.Duration{
		"hourly":  time.Hour,
		"daily":   24 * time.Hour,
		"weekly":  7 * 24 * time.Hour,
		"monthly": 31 * 24 * time.Hour,
		"unknown": 24 * time.Hour, // default daily
	}
	for freq, want := range cases {
		if got := frequencyInterval(freq); got != want {
			t.Errorf("frequencyInterval(%q)=%v want %v", freq, got, want)
		}
	}
}
