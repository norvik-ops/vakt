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

// TestBuildEmail_ISMSOnly covers the R1-KW-15 follow-up: the digest reports
// risks and CAPAs, and no findings or DSR section of the archived Scan and
// Privacy modules.
func TestBuildEmail_ISMSOnly(t *testing.T) {
	content := digestContent{Comply: complySummary{OpenRisks: 3, OpenCAPAs: 2}}
	_, body := buildEmail("org-1", content, "de")
	assert.Contains(t, body, "Offene Risiken</td><td>3")
	assert.Contains(t, body, "Offene CAPAs</td><td>2")
	assert.NotContains(t, body, "Findings")
	assert.NotContains(t, body, "DSR")
}
