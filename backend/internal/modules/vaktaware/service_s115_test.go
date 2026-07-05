// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktaware

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── landingPagePolicy (Stored-XSS prevention) ────────────────────────────────

func TestLandingPagePolicy_StripScript(t *testing.T) {
	out := landingPagePolicy.Sanitize(`<script>alert(1)</script><p>safe</p>`)
	assert.NotContains(t, out, "<script>")
	assert.Contains(t, out, "<p>safe</p>")
}

func TestLandingPagePolicy_StripOnclick(t *testing.T) {
	out := landingPagePolicy.Sanitize(`<a href="http://example.com" onclick="evil()">click</a>`)
	assert.NotContains(t, out, "onclick")
	assert.Contains(t, out, `href="http://example.com"`)
}

func TestLandingPagePolicy_StripOnerror(t *testing.T) {
	out := landingPagePolicy.Sanitize(`<img src="x" onerror="alert(1)">`)
	assert.NotContains(t, out, "onerror")
}

func TestLandingPagePolicy_AllowsStyledDivs(t *testing.T) {
	// Branded landing pages need id/class/style on structural elements.
	out := landingPagePolicy.Sanitize(`<div id="main" class="container" style="color:red">hello</div>`)
	assert.Contains(t, out, `id="main"`)
	assert.Contains(t, out, `class="container"`)
}

func TestLandingPagePolicy_AllowsLinks(t *testing.T) {
	out := landingPagePolicy.Sanitize(`<a href="https://example.com">click</a>`)
	assert.Contains(t, out, `href="https://example.com"`)
}

// ── LaunchCampaign SMTP guard ─────────────────────────────────────────────────

func TestLaunchCampaign_NoSMTPConfigured(t *testing.T) {
	// smtpCfg.Host is empty — must return error before any DB call.
	svc := &Service{}
	err := svc.LaunchCampaign(context.Background(), "org1", "campaign1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SMTP")
}

// ── EnqueueAutoEnrollment nil client ─────────────────────────────────────────

func TestEnqueueAutoEnrollment_NilClientReturnsNil(t *testing.T) {
	// asynqClient is nil — no error, no panic (Asynq not configured).
	svc := &Service{}
	err := svc.EnqueueAutoEnrollment(context.Background(), AutoEnrollmentPayload{
		OrgID: "org1", TriggerType: "new_employee",
	})
	assert.NoError(t, err)
}

// ── ImportTargetsCSV — header skip + empty-line tolerance ────────────────────

func TestImportTargetsCSV_HeaderOnlyReturnsZero(t *testing.T) {
	// Header row is skipped, blank lines are skipped — no repo call.
	svc := &Service{}
	imported, errs := svc.ImportTargetsCSV(context.Background(), "org1", "grp1", "email,first,last\n\n")
	assert.Equal(t, 0, imported)
	assert.Empty(t, errs)
}

func TestImportTargetsCSV_EmptyCSVReturnsZero(t *testing.T) {
	svc := &Service{}
	imported, errs := svc.ImportTargetsCSV(context.Background(), "org1", "grp1", "")
	assert.Equal(t, 0, imported)
	assert.Empty(t, errs)
}
