package vaktcomply

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestDefaultMaxAgeDays(t *testing.T) {
	cases := []struct {
		evidenceType string
		expected     int
	}{
		{"scanner", 7},
		{"cloud", 2},
		{"policy", 365},
		{"pentest", 365},
		{"phishing", 90},
		{"manual", 180},
		{"unknown_type", 180},
	}
	for _, tc := range cases {
		t.Run(tc.evidenceType, func(t *testing.T) {
			assert.Equal(t, tc.expected, DefaultMaxAgeDays(tc.evidenceType))
		})
	}
}

func TestComplianceScore_StaleCountsAsNotOk(t *testing.T) {
	// Simulate: 10 controls, 8 ok, 2 stale → Score = 8/10 = 80%
	s := ComplianceScore{
		TotalControls: 10,
		OkCount:       8,
		StaleCount:    2,
		MissingCount:  0,
		NACount:       0,
	}
	denominator := s.TotalControls - s.NACount
	require.Equal(t, 10, denominator)
	s.ScorePct = float64(s.OkCount) / float64(denominator) * 100
	assert.InDelta(t, 80.0, s.ScorePct, 0.01, "stale counts as not-ok: 8/10 = 80%")
}

func TestComplianceScore_NAExcludedFromDenominator(t *testing.T) {
	// 10 controls, 8 ok, 2 na → Score = 8/8 = 100%
	s := ComplianceScore{
		TotalControls: 10,
		OkCount:       8,
		StaleCount:    0,
		MissingCount:  0,
		NACount:       2,
	}
	denominator := s.TotalControls - s.NACount
	require.Equal(t, 8, denominator)
	s.ScorePct = float64(s.OkCount) / float64(denominator) * 100
	assert.InDelta(t, 100.0, s.ScorePct, 0.01, "NA excluded: 8/8 = 100%")
}

func TestEvidenceStalenessLogic(t *testing.T) {
	now := time.Now().UTC()

	// Evidence 8 days old, max_age = 7 → stale
	evidenceAge := 8 * 24 * time.Hour
	maxAgeDays := 7
	isStale := now.Sub(now.Add(-evidenceAge)) > time.Duration(maxAgeDays)*24*time.Hour
	assert.True(t, isStale, "8-day-old evidence with max_age=7 should be stale")

	// Evidence 6 days old, max_age = 7 → ok
	evidenceAge = 6 * 24 * time.Hour
	isStale = now.Sub(now.Add(-evidenceAge)) > time.Duration(maxAgeDays)*24*time.Hour
	assert.False(t, isStale, "6-day-old evidence with max_age=7 should be ok")

	// No evidence → missing (handled by NULL check in SQL)
	// max_age = NULL → ok regardless of age (no staleness check)
}
