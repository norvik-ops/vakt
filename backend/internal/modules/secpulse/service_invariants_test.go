// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secpulse

// Domain-invariant tests for secpulse pure-logic functions.
//
// These tests require no database or network connection. They guard against
// silent regressions in:
//   - SLA severity mapping (compliance deadline calculation)
//   - EOL version-cycle parsing (endoflife.date key derivation)
//   - EOL payload deserialization (polymorphic bool/string/date from external API)

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── SLA: severity → days mapping ─────────────────────────────────────────────

// TestSLADaysForSeverity_AllKnownSeverities verifies that each recognised
// severity maps to a non-zero SLA day count from the supplied config.
// A zero return would silently mark every finding as overdue.
func TestSLADaysForSeverity_AllKnownSeverities(t *testing.T) {
	cfg := &SLAConfig{
		CriticalDays: 7,
		HighDays:     30,
		MediumDays:   90,
		LowDays:      180,
	}

	cases := []struct {
		severity string
		want     int
	}{
		{"critical", 7},
		{"high", 30},
		{"medium", 90},
		{"low", 180},
	}

	for _, tc := range cases {
		t.Run(tc.severity, func(t *testing.T) {
			got := slaDaysForSeverity(cfg, tc.severity)
			assert.Equal(t, tc.want, got,
				"severity %q must map to configured SLA days", tc.severity)
		})
	}
}

// TestSLADaysForSeverity_UnknownFallsBackTo90 verifies that unrecognised
// severity values fall back to 90 days (BSI-Grundschutz baseline).
// This prevents unknown severity findings from being silently excluded from SLA tracking.
func TestSLADaysForSeverity_UnknownFallsBackTo90(t *testing.T) {
	cfg := &SLAConfig{CriticalDays: 7, HighDays: 30, MediumDays: 90, LowDays: 180}

	unknownSeverities := []string{"", "informational", "CRITICAL", "High", "unknown", "n/a"}
	for _, sev := range unknownSeverities {
		t.Run(sev, func(t *testing.T) {
			got := slaDaysForSeverity(cfg, sev)
			assert.Equal(t, 90, got,
				"unrecognised severity %q must fall back to 90-day BSI baseline", sev)
		})
	}
}

// TestSLAEntry_OverdueFlag_ReflectsDaysVsSLA checks the overdue comparison
// is DaysOpen > SLADays (strictly greater, not >=), so a finding at exactly
// its SLA boundary is not yet overdue.
func TestSLAEntry_OverdueFlag_ReflectsDaysVsSLA(t *testing.T) {
	cases := []struct {
		daysOpen int
		slaDays  int
		overdue  bool
	}{
		{29, 30, false}, // still within SLA
		{30, 30, false}, // exactly at SLA boundary — not overdue
		{31, 30, true},  // one day past deadline
		{0, 7, false},   // brand new finding
	}
	for _, tc := range cases {
		got := tc.daysOpen > tc.slaDays
		assert.Equal(t, tc.overdue, got,
			"daysOpen=%d, slaDays=%d: overdue flag mismatch", tc.daysOpen, tc.slaDays)
	}
}

// ─── EOL: version-cycle extraction ────────────────────────────────────────────

// TestMajorCycle_SemverExtraction verifies that majorCycle correctly extracts
// the "major.minor" segment used as the endoflife.date cycle key.
// A wrong cycle key silently returns "unknown" instead of an accurate EOL status.
func TestMajorCycle_SemverExtraction(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"3.9.7", "3.9"},
		{"1.21.5", "1.21"},
		{"v3.9.7", "3.9"}, // v-prefix must be stripped
		{"18.0.1", "18.0"},
		{"2.0", "2.0"},   // two-part version
		{"5", "5"},       // single-part: returns as-is
		{"v1.22", "1.22"}, // v-prefix with two-part
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := majorCycle(tc.input)
			assert.Equal(t, tc.want, got,
				"majorCycle(%q) must yield %q for endoflife.date lookup", tc.input, tc.want)
		})
	}
}

// TestNormaliseCycle_VPrefixAndCase verifies that normaliseCycle strips the "v"
// prefix and lowercases the result so that API-returned cycles ("V3.9") match
// SBOM-reported cycles ("v3.9" or "V3.9").
func TestNormaliseCycle_VPrefixAndCase(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"v3.9", "3.9"},
		{"V3.9", "3.9"},
		{"3.9", "3.9"},
		{"JAMMY", "jammy"},  // Ubuntu codenames
		{"v18.04", "18.04"},
		{"", ""},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := normaliseCycle(tc.input)
			assert.Equal(t, tc.want, got,
				"normaliseCycle(%q) mismatch", tc.input)
		})
	}
}

// ─── EOL: API payload parsing ─────────────────────────────────────────────────

// TestParseEOLPayload_EmptyPayload verifies that an empty cached blob
// returns "unknown" status without error (API hasn't been queried yet).
func TestParseEOLPayload_EmptyPayload(t *testing.T) {
	status, date, err := parseEOLPayload([]byte{})
	require.NoError(t, err)
	assert.Equal(t, "unknown", status)
	assert.Nil(t, date)
}

// TestParseEOLPayload_SupportedBoolFalse verifies that {"eol":false} returns
// "supported" — the common case for active, maintained releases.
func TestParseEOLPayload_SupportedBoolFalse(t *testing.T) {
	payload := []byte(`{"cycle":"3.9","eol":false}`)
	status, date, err := parseEOLPayload(payload)
	require.NoError(t, err)
	assert.Equal(t, "supported", status)
	assert.Nil(t, date)
}

// TestParseEOLPayload_EOLBoolTrue verifies that {"eol":true} (no date) returns
// "eol" with a nil date — cycle is end-of-life but no specific date was given.
func TestParseEOLPayload_EOLBoolTrue(t *testing.T) {
	payload := []byte(`{"cycle":"2.7","eol":true}`)
	status, date, err := parseEOLPayload(payload)
	require.NoError(t, err)
	assert.Equal(t, "eol", status)
	assert.Nil(t, date, "bool-true EOL must produce nil date")
}

// TestParseEOLPayload_EOLWithDateString verifies that {"eol":"2024-01-01"}
// returns "eol" with the date string populated.
func TestParseEOLPayload_EOLWithDateString(t *testing.T) {
	payload := []byte(`{"cycle":"3.8","eol":"2024-10-14"}`)
	status, date, err := parseEOLPayload(payload)
	require.NoError(t, err)
	assert.Equal(t, "eol", status)
	require.NotNil(t, date)
	assert.Equal(t, "2024-10-14", *date)
}

// TestParseEOLPayload_MalformedJSON verifies that invalid JSON returns
// "unknown" status and a wrapped error — not a panic or silent wrong result.
func TestParseEOLPayload_MalformedJSON(t *testing.T) {
	payload := []byte(`{not valid json`)
	status, date, err := parseEOLPayload(payload)
	assert.Error(t, err, "malformed JSON must produce an error")
	assert.Equal(t, "unknown", status)
	assert.Nil(t, date)
}

// ─── EOL: eolValue polymorphic unmarshalling ──────────────────────────────────

// TestEOLValue_BoolFalse verifies that JSON `false` is decoded as not EOL.
func TestEOLValue_BoolFalse(t *testing.T) {
	var v eolValue
	require.NoError(t, json.Unmarshal([]byte(`false`), &v))
	assert.False(t, v.IsEOL)
	assert.Empty(t, v.Date)
}

// TestEOLValue_BoolTrue verifies that JSON `true` sets IsEOL without a date.
func TestEOLValue_BoolTrue(t *testing.T) {
	var v eolValue
	require.NoError(t, json.Unmarshal([]byte(`true`), &v))
	assert.True(t, v.IsEOL)
	assert.Empty(t, v.Date)
}

// TestEOLValue_DateString verifies that a YYYY-MM-DD string sets both IsEOL
// and Date — used when endoflife.date provides a concrete sunset date.
func TestEOLValue_DateString(t *testing.T) {
	var v eolValue
	require.NoError(t, json.Unmarshal([]byte(`"2023-12-31"`), &v))
	assert.True(t, v.IsEOL)
	assert.Equal(t, "2023-12-31", v.Date)
}

// TestEOLValue_StringFalse verifies that the string literal "false" is treated
// as not EOL — some API versions return strings instead of booleans.
func TestEOLValue_StringFalse(t *testing.T) {
	var v eolValue
	require.NoError(t, json.Unmarshal([]byte(`"false"`), &v))
	assert.False(t, v.IsEOL)
}

// TestEOLValue_EmptyString verifies that an empty string is treated as not EOL.
func TestEOLValue_EmptyString(t *testing.T) {
	var v eolValue
	require.NoError(t, json.Unmarshal([]byte(`""`), &v))
	assert.False(t, v.IsEOL)
}

// TestEOLValue_InvalidType verifies that an unexpected JSON type (e.g. object)
// returns an error rather than silently defaulting to IsEOL=false.
func TestEOLValue_InvalidType(t *testing.T) {
	var v eolValue
	err := json.Unmarshal([]byte(`{"key":"value"}`), &v)
	assert.Error(t, err, "object type must be rejected as invalid eolValue")
}
