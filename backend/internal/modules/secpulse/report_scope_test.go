// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secpulse

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReportScope_RoundTrip — typed struct survives a full JSON marshal → unmarshal cycle.
func TestReportScope_RoundTrip(t *testing.T) {
	original := Report{
		ID:    "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		OrgID: "org-uuid",
		Title: "Executive Report Q2-2026",
		Scope: ReportScope{Title: "Executive Report Q2-2026"},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err, "marshal Report should not fail")

	var restored Report
	require.NoError(t, json.Unmarshal(data, &restored), "unmarshal Report should not fail")
	assert.Equal(t, original.Scope.Title, restored.Scope.Title)
}

// TestReportScope_ZeroValue — empty ReportScope marshals without error.
func TestReportScope_ZeroValue(t *testing.T) {
	r := Report{ID: "zero-id", OrgID: "zero-org", Status: "pending"}

	data, err := json.Marshal(r)
	require.NoError(t, err, "marshal zero-value Report should not fail")
	assert.NotEmpty(t, data)

	var restored Report
	require.NoError(t, json.Unmarshal(data, &restored))
	assert.Equal(t, "", restored.Scope.Title)
}

// TestReportScope_UnknownFields — extra JSON keys in scope are silently ignored (forward-compat).
func TestReportScope_UnknownFields(t *testing.T) {
	raw := `{
		"id":     "report-uuid",
		"org_id": "org-uuid",
		"status": "completed",
		"scope":  {"title": "My Report", "future_field_v2": "ignored", "nested": {"key": "val"}}
	}`

	var r Report
	err := json.Unmarshal([]byte(raw), &r)
	require.NoError(t, err, "unknown JSON fields should be silently ignored")
	assert.Equal(t, "My Report", r.Scope.Title)
}

// TestReportScope_TitleIsString — compile-time assertion: Title is a typed string, not interface{}.
func TestReportScope_TitleIsString(t *testing.T) {
	s := ReportScope{Title: "Typed"}
	var _ string = s.Title //nolint:staticcheck // QF1011: explicit type is the compile-time assertion
	assert.Equal(t, "Typed", s.Title)
}

// TestReportScope_WireFormat — confirms JSON wire shape is {"title":"..."} for Asynq compatibility.
// Existing enqueued payloads with {"title":"..."} must unmarshal cleanly after the typed migration.
func TestReportScope_WireFormat(t *testing.T) {
	s := ReportScope{Title: "Wire Format Test"}
	data, err := json.Marshal(s)
	require.NoError(t, err)
	assert.JSONEq(t, `{"title":"Wire Format Test"}`, string(data))
}

// TestReportScope_LegacyPayloadCompat — a raw Asynq payload from before the typed migration
// must still unmarshal into the new struct without error.
func TestReportScope_LegacyPayloadCompat(t *testing.T) {
	legacyJSON := `{"report_id":"abc","org_id":"org1","scope":{"title":"Legacy Report"}}`

	var p GenerateReportPayload
	require.NoError(t, json.Unmarshal([]byte(legacyJSON), &p))
	assert.Equal(t, "Legacy Report", p.Scope.Title)
}

// TestReportScope_Title_SpecialChars_MarshalsSafely verifies that adversarial Title
// values (XSS, SQL injection, path traversal, template injection) survive a JSON
// round-trip through ReportScope unchanged. Sanitisation is the renderer's job.
func TestReportScope_Title_SpecialChars_MarshalsSafely(t *testing.T) {
	adversarialTitles := []string{
		"<script>alert(1)</script>",
		"'; DROP TABLE findings;--",
		"../../etc/passwd",
		`" onmouseover="alert(1)`,
		"<img src=x onerror=alert(1)>",
		"{{ 7*7 }}",
		"\x00\x01\x1f",
		"\u200b",
	}
	for _, title := range adversarialTitles {
		title := title
		t.Run(title, func(t *testing.T) {
			s := ReportScope{Title: title}
			data, err := json.Marshal(s)
			require.NoError(t, err)
			var restored ReportScope
			require.NoError(t, json.Unmarshal(data, &restored))
			assert.Equal(t, title, restored.Title,
				"ReportScope.Title must survive JSON round-trip unchanged — HTML escaping is the renderer's job")
		})
	}
}

// TestGenerateReportPayload_MaliciousTitle_RoundTrip verifies that a GenerateReportPayload
// with a malicious title faithfully carries it through a JSON round-trip (Asynq queue payload).
func TestGenerateReportPayload_MaliciousTitle_RoundTrip(t *testing.T) {
	maliciousTitles := []string{
		"<script>alert(document.cookie)</script>",
		"'; DROP TABLE so_secrets;--",
		"../../../etc/shadow",
		`"><svg/onload=alert(1)>`,
	}
	for _, title := range maliciousTitles {
		title := title
		t.Run(title, func(t *testing.T) {
			original := GenerateReportPayload{
				ReportID: "rpt-uuid-001",
				OrgID:    "org-uuid-001",
				Scope:    ReportScope{Title: title},
			}
			data, err := json.Marshal(original)
			require.NoError(t, err)
			var restored GenerateReportPayload
			require.NoError(t, json.Unmarshal(data, &restored))
			assert.Equal(t, title, restored.Scope.Title)
			assert.Equal(t, original.ReportID, restored.ReportID)
		})
	}
}
