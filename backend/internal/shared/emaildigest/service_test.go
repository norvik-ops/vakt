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
