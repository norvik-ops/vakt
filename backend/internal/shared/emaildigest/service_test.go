// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package emaildigest

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSendAIDigestEmail_NoSMTP verifies that SendAIDigestEmail is a clean no-op
// (no error, no panic) when SMTP is not configured.
func TestSendAIDigestEmail_NoSMTP(t *testing.T) {
	err := SendAIDigestEmail(context.Background(), nil, SMTPConfig{}, "org-1", "Acme GmbH", "AI narrative")
	require.NoError(t, err)
}

// TestBuildAIDigestBody verifies the HTML body contains the narrative and org name.
func TestBuildAIDigestBody(t *testing.T) {
	body := buildAIDigestBody("Ihr Compliance-Score ist stabil.", "Acme GmbH")
	assert.Contains(t, body, "Ihr Compliance-Score ist stabil.")
	assert.Contains(t, body, "Acme GmbH")
	assert.True(t, strings.HasPrefix(body, "<!DOCTYPE html>"), "body must be valid HTML")
}

// TestBuildEmail_OnlyEnabledModules covers the R1-KW-15 follow-up: with only the
// ISMS enabled the digest reports risks and CAPAs, and no findings or DSR section
// for the switched-off Scan and Privacy modules.
func TestBuildEmail_OnlyEnabledModules(t *testing.T) {
	ismsOnly := digestContent{Comply: complySummary{OpenRisks: 3, OpenCAPAs: 2}}
	_, body := buildEmail("org-1", ismsOnly, "de")
	assert.Contains(t, body, "Offene Risiken</td><td>3")
	assert.Contains(t, body, "Offene CAPAs</td><td>2")
	assert.NotContains(t, body, "Offene Findings")
	assert.NotContains(t, body, "DSR-Anfragen")

	all := digestContent{
		Comply:       complySummary{OpenRisks: 1},
		ShowFindings: true,
		Findings:     []findingSeverityCount{{Severity: "critical", Count: 4}},
		ShowDSR:      true,
		DSR:          dsrSummary{Overdue: 1},
	}
	_, body = buildEmail("org-1", all, "de")
	assert.Contains(t, body, "Offene Findings")
	assert.Contains(t, body, ">critical</td><td>4")
	assert.Contains(t, body, "DSR-Anfragen")
}
